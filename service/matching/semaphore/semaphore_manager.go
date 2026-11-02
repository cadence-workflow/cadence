package semaphore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/locks"
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
//
// Three locks, taken in this order:
//   - bucketLock: one token write at a time. Held across the database write.
//   - the cache lock (bucketCache.cacheMu): the token states. Never held across I/O.
//   - stateMu: the lifecycle state and the current cache. Never held while taking another lock.
//
// It decides what each database answer means for the cache. A token another owner may hold is
// marked unavailable. A token whose write failed is left free, since the write may have landed and
// a token wrongly marked free costs only a refused write.
type semaphoreManagerImpl struct {
	id     Identifier
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

	// bucketLock lets one token write run at a time.
	bucketLock locks.Mutex

	// stateMu guards state and cache.
	stateMu sync.Mutex
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
		bucketLock:    locks.NewMutex(),
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

// Start loads the bucket's cache.
func (m *semaphoreManagerImpl) Start(ctx context.Context) error {
	m.stateMu.Lock()
	found := m.state
	if found == managerStateCreated {
		m.state = managerStateStarting
	}
	m.stateMu.Unlock()

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
	// Counted before the cache is shared.
	freeTokens, heldTokens := cache.freeTokenCount(), cache.heldTokenCount()

	m.stateMu.Lock()
	if m.state == managerStateStopped {
		m.stateMu.Unlock()
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
	m.stateMu.Unlock()

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

// Stop shuts the manager down: later acquires and releases get ErrNotReady. A grant
// already past the state check still finishes its write.
func (m *semaphoreManagerImpl) Stop() {
	m.stopOnce.Do(func() {
		m.stateMu.Lock()
		m.state = managerStateStopped
		m.stateMu.Unlock()

		// Unregistered before the rest of the teardown, so nothing is handed a manager that can
		// no longer serve.
		m.onStopFn(m)
		m.liveness.Stop()

		m.markStartupDone()
		m.logger.Info("Semaphore manager stopped", tag.LifeCycleStopped)
	})
}

func (m *semaphoreManagerImpl) isRunning() bool {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.state == managerStateRunning
}

// currentCache returns the bucket's cache, which is empty until the load finishes. Writes call it
// after taking the bucket lock.
func (m *semaphoreManagerImpl) currentCache() *bucketCache {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.cache
}

// Acquire is the entry point: it asks for a token on behalf of ownerID, and queues a waiter when
// this host cannot find one.
func (m *semaphoreManagerImpl) Acquire(ctx context.Context, ownerID string) (AcquireResult, error) {
	m.liveness.MarkAlive()
	if err := m.awaitStartup(ctx); err != nil {
		return AcquireResult{}, err
	}

	tokenID, err := m.grant(ctx, ownerID)
	if err != nil {
		return AcquireResult{}, err
	}
	if tokenID != emptyTokenID {
		return AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: tokenID}, nil
	}
	// TODO: Once enqueue writes a waiter, answer QUEUED here, since NO_SLOT means nothing is
	// queued. Before enqueuing, refresh the free tokens and retry the grant once: a stale cache
	// can answer NO_SLOT while tokens are free, and a waiter queued on that answer may never be
	// woken.
	if err := m.enqueue(ctx, ownerID); err != nil {
		return AcquireResult{}, err
	}
	return AcquireResult{Outcome: types.SemaphoreAcquireOutcomeNoSlot}, nil
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

// grant returns the token ownerID holds, or grants a free one, or returns emptyTokenID if no token
// is free.
func (m *semaphoreManagerImpl) grant(ctx context.Context, ownerID string) (int, error) {
	// Fast path: a retried acquire whose token is cached and confirmed. No lock.
	heldTokenID, err := m.confirmCachedToken(ctx, ownerID)
	if err != nil || heldTokenID != emptyTokenID {
		return heldTokenID, err
	}

	// Slow path: under the bucket lock, check again (a grant for ownerID may have landed while
	// waiting, and only here can the cache be fixed), then try free tokens.
	if err := m.bucketLock.Lock(ctx); err != nil {
		return emptyTokenID, err
	}
	defer m.bucketLock.Unlock()
	cache := m.currentCache()

	heldTokenID, err = m.refreshCachedTokenLocked(ctx, cache, ownerID)
	if err != nil || heldTokenID != emptyTokenID {
		return heldTokenID, err
	}
	return m.grantFreeTokenLocked(ctx, cache, ownerID)
}

// confirmCachedToken returns ownerID's cached token if the database confirms it, or
// emptyTokenID. It takes no lock, so it never writes the cache.
func (m *semaphoreManagerImpl) confirmCachedToken(ctx context.Context, ownerID string) (int, error) {
	tokenID, ok := m.currentCache().lookupHeldToken(ownerID)
	if !ok {
		return emptyTokenID, nil
	}
	row, err := m.db.getTokenRow(ctx, tokenID)
	if err != nil {
		return emptyTokenID, err
	}
	if row == nil || row.Holder != ownerID {
		return emptyTokenID, nil
	}
	return tokenID, nil
}

// refreshCachedTokenLocked returns ownerID's cached token if the database confirms it, or
// emptyTokenID. It fixes the cache from the database. The caller holds the bucket lock.
func (m *semaphoreManagerImpl) refreshCachedTokenLocked(ctx context.Context, cache *bucketCache, ownerID string) (int, error) {
	tokenID, ok := cache.lookupHeldToken(ownerID)
	if !ok {
		return emptyTokenID, nil
	}
	row, err := m.db.getTokenRow(ctx, tokenID)
	if err != nil {
		return emptyTokenID, err
	}
	if row == nil {
		// This should not happen, since a grant writes the token row together with the owner row
		// and nothing deletes it.
		m.logger.Warn("Semaphore owner's token has no token row", tag.Dynamic("token-id", tokenID))
		cache.markTokenUnavailable(tokenID)
		return emptyTokenID, nil
	}
	switch row.Holder {
	case ownerID:
		cache.markTokenHeld(tokenID, ownerID)
		return tokenID, nil
	case "":
		// TODO: Signal the waiter reader, so it grants the freed token to the first waiter.
		cache.markTokenFree(tokenID)
		return emptyTokenID, nil
	default:
		// Another owner holds tokenID. Owners are recorded only from owner rows, as at load.
		cache.markTokenUnavailable(tokenID)
		return emptyTokenID, nil
	}
}

// grantFreeTokenLocked grants ownerID a free token, trying up to maxGrantAttempts tokens, or
// returns emptyTokenID. The caller holds the bucket lock.
func (m *semaphoreManagerImpl) grantFreeTokenLocked(ctx context.Context, cache *bucketCache, ownerID string) (int, error) {
	for range maxGrantAttempts {
		// Stop if the caller gave up. The write would fail with a persistence.TimeoutError
		// (deadline) or an InternalServiceError (cancel), and neither says the caller gave up.
		if err := ctx.Err(); err != nil {
			return emptyTokenID, err
		}

		pickedTokenID, ok := cache.pickFreeToken()
		if !ok {
			return emptyTokenID, nil
		}
		heldTokenID, err := m.tryGrantTokenLocked(ctx, cache, pickedTokenID, ownerID)
		if err != nil || heldTokenID != emptyTokenID {
			return heldTokenID, err
		}
	}
	return emptyTokenID, nil
}

// tryGrantTokenLocked writes ownerID's grant of tokenID and returns the token ownerID now holds, or
// emptyTokenID if another owner holds tokenID. The caller holds the bucket lock.
func (m *semaphoreManagerImpl) tryGrantTokenLocked(ctx context.Context, cache *bucketCache, tokenID int, ownerID string) (heldTokenID int, err error) {
	if !cache.tokenRange.Contains(tokenID) {
		m.logger.Error("Semaphore grant picked a token this bucket does not own",
			tag.Dynamic("token-id", tokenID),
			tag.Dynamic("first-token", cache.tokenRange.First),
			tag.Dynamic("last-token", cache.tokenRange.Last()))
		return emptyTokenID, &types.InternalServiceError{Message: fmt.Sprintf("token %d is outside the range of bucket %v", tokenID, m.id)}
	}

	resp, err := m.db.grantToken(ctx, tokenID, ownerID)
	if err != nil {
		// Nothing is recorded, so tokenID stays free. If the write did land, the database refuses
		// the next grant on it and the cache marks it unavailable.
		return emptyTokenID, err
	}
	switch resp.Outcome {
	case persistence.SemaphoreGrantApplied:
		cache.markTokenHeld(tokenID, ownerID)
		return tokenID, nil

	case persistence.SemaphoreGrantSlotTaken:
		// Another owner holds tokenID, so the cache was stale.
		cache.markTokenUnavailable(tokenID)
		return emptyTokenID, nil

	case persistence.SemaphoreGrantAlreadyHeld:
		// The owner already holds a token, so answer with it. tokenID stays free.
		if !cache.tokenRange.Contains(resp.HeldToken) {
			// The owner row names a token this bucket does not own, or no token at all, so it is
			// corrupt. Answering with it would hand out a token outside the bucket. Every acquire
			// for this owner fails this way until the owner row is fixed.
			return emptyTokenID, &types.InternalServiceError{Message: fmt.Sprintf("grant reported AlreadyHeld with token %d, outside tokens %d to %d of bucket %v", resp.HeldToken, cache.tokenRange.First, cache.tokenRange.Last(), m.id)}
		}
		cache.markTokenHeld(resp.HeldToken, ownerID)
		return resp.HeldToken, nil

	default:
		// Unreachable through the nosql store, which rejects unknown outcomes itself, but that is
		// one store's guarantee, not the interface's, so check anyway.
		return emptyTokenID, &types.InternalServiceError{Message: fmt.Sprintf("unexpected grant outcome %v for bucket %v", resp.Outcome, m.id)}
	}
}

// Release frees the token ownerID holds, trying the cached token first and then the one its owner
// row names. Releasing an owner that holds nothing succeeds, so a repeated release succeeds.
func (m *semaphoreManagerImpl) Release(ctx context.Context, ownerID string) error {
	m.liveness.MarkAlive()
	if err := m.awaitStartup(ctx); err != nil {
		return err
	}

	if err := m.bucketLock.Lock(ctx); err != nil {
		return err
	}
	defer m.bucketLock.Unlock()
	cache := m.currentCache()

	// First, release the token the cache has for ownerID.
	released, err := m.releaseCachedTokenLocked(ctx, cache, ownerID)
	if err != nil {
		return err
	}
	if released {
		return nil
	}
	// The cache has no token for ownerID, or it is stale. Read ownerID's owner row and try to
	// release the token it names, if any.
	return m.releaseOwnerRowTokenLocked(ctx, cache, ownerID)
}

// releaseCachedTokenLocked releases ownerID's cached token. It returns false if the cache has no
// token for ownerID or the release did not apply. The caller holds the bucket lock.
func (m *semaphoreManagerImpl) releaseCachedTokenLocked(ctx context.Context, cache *bucketCache, ownerID string) (released bool, err error) {
	tokenID, ok := cache.lookupHeldToken(ownerID)
	if !ok {
		return false, nil
	}
	return m.tryReleaseTokenLocked(ctx, cache, tokenID, ownerID)
}

// releaseOwnerRowTokenLocked reads ownerID's owner row and releases the token in it. The caller holds the bucket lock.
func (m *semaphoreManagerImpl) releaseOwnerRowTokenLocked(ctx context.Context, cache *bucketCache, ownerID string) error {
	row, err := m.db.getOwnerRow(ctx, ownerID)
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	tokenID := row.HeldToken
	if !cache.tokenRange.Contains(tokenID) {
		// The cache has no entry for tokenID, so marking it after the release would panic.
		m.logger.Error("Semaphore owner row names a token this bucket does not own",
			tag.Dynamic("token-id", tokenID),
			tag.Dynamic("first-token", cache.tokenRange.First),
			tag.Dynamic("last-token", cache.tokenRange.Last()))
		return &types.InternalServiceError{Message: fmt.Sprintf("owner %v holds token %d, which is outside the range of bucket %v", ownerID, tokenID, m.id)}
	}
	applied, err := m.tryReleaseTokenLocked(ctx, cache, tokenID, ownerID)
	if err != nil {
		return err
	}
	if !applied {
		// The owner row names tokenID, but tokenID's token row does not name ownerID. This should
		// not happen, since a grant and a release each write both rows together. The error is
		// retryable, so History retries the release.
		m.logger.Warn("Semaphore owner row names a token whose token row does not name the owner",
			tag.Dynamic("token-id", tokenID))
		return &types.InternalServiceError{Message: fmt.Sprintf("owner %v's owner row names token %d of bucket %v, but the token row does not name the owner", ownerID, tokenID, m.id)}
	}
	return nil
}

// tryReleaseTokenLocked releases ownerID's tokenID.  The caller holds the bucket lock.
func (m *semaphoreManagerImpl) tryReleaseTokenLocked(ctx context.Context, cache *bucketCache, tokenID int, ownerID string) (applied bool, err error) {
	applied, err = m.db.releaseToken(ctx, tokenID, ownerID)
	if err != nil {
		// The write may have landed, so the token may be free. If it did not land, the next grant
		// of the token is refused and marks it unavailable.
		cache.markTokenFree(tokenID)
		return false, err
	}
	if !applied {
		cache.markTokenUnavailable(tokenID)
		return false, nil
	}
	cache.markTokenFree(tokenID)
	return true, nil
}
