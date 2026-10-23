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
	// tokenRange is the token ids this bucket owns. Load sets it once before startup ends, so
	// requests read it without mu.
	tokenRange commonsemaphore.TokenRange

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
	// Counted before the cache is shared, so no bucket lock is needed.
	freeTokens, heldTokens := cache.freeTokenCount(), cache.ownerCount()

	m.mu.Lock()
	if m.state == managerStateStopped {
		m.mu.Unlock()
		// Stop landed during the scan, so this host no longer owns the bucket and the scan
		// result is already stale.
		return fmt.Errorf("semaphore manager %v was stopped while it was loading: %w", m.id, ErrNotReady)
	}
	m.tokenRange = tokenRange
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
	if ownerID == "" {
		return AcquireResult{}, &types.BadRequestError{Message: "ownerID is required"}
	}
	// Marked before the startup wait, so a bucket whose first request arrives during a slow
	// scan is not counted as idle the moment it finishes loading.
	m.liveness.MarkAlive()

	// Wait for startup to finish before reading any state, since there are no free tokens until
	// the scan fills it.
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

// enqueue records ownerID as a waiter on this bucket, to be granted a token when one frees.
// Acquire calls it when a grant finds the bucket full.
//
// TODO: Not built yet, so it queues nothing. It will write a waiter row to semaphore_tasks under
// the bucket's range_id, with a TTL taken from the acquire deadline, and a background reader will
// grant waiters in order as tokens are released.
func (m *semaphoreManagerImpl) enqueue(ctx context.Context, ownerID string) error {
	return nil
}

// grant gets ownerID a token:
//   - If ownerID already holds a token, it answers Acquired with that token, so a retried acquire
//     gets the same answer.
//   - Otherwise it grants a free token and answers Acquired with it, or NoSlot if none could be
//     granted.
func (m *semaphoreManagerImpl) grant(ctx context.Context, ownerID string) (AcquireResult, error) {
	cache := m.currentCache()
	heldTokenID, found, err := m.findHeldToken(ctx, cache, ownerID)
	if err != nil {
		return AcquireResult{}, err
	}
	if found {
		return AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: heldTokenID}, nil
	}
	return m.grantFreeToken(ctx, cache, ownerID)
}

// findHeldToken returns the token ownerID holds: it looks the token up in the cache and confirms
// it in the database. found is false if the cache has no token for ownerID, or the database says
// ownerID does not hold it.
func (m *semaphoreManagerImpl) findHeldToken(ctx context.Context, cache *bucketCache, ownerID string) (tokenID int, found bool, err error) {
	if err := cache.lock(ctx); err != nil {
		return 0, false, err
	}
	defer cache.unlock()

	tokenID, ok := cache.lookupHeldToken(ownerID)
	if !ok {
		return 0, false, nil
	}
	row, rowFound, err := m.db.getTokenRow(ctx, tokenID)
	if err != nil {
		return 0, false, err
	}
	if !rowFound {
		// This should not happen, since a grant writes the token row together with the owner row
		// and nothing deletes it.
		m.logger.Warn("Semaphore owner's token has no token row", tag.Dynamic("token-id", tokenID))
	}
	cache.updateFromTokenRow(ownerID, tokenID, row)
	if !rowFound || row.Holder != ownerID {
		return 0, false, nil
	}
	return tokenID, true, nil
}

// grantFreeToken grants ownerID a free token, up to maxGrantAttempts times. A write refused because
// another owner holds the token costs an attempt, and a different token is tried.
func (m *semaphoreManagerImpl) grantFreeToken(ctx context.Context, cache *bucketCache, ownerID string) (AcquireResult, error) {
	for range maxGrantAttempts {
		// Stop if the caller gave up. Otherwise the write fails with a persistence.TimeoutError,
		// which does not say whether the caller or the store timed out.
		if err := ctx.Err(); err != nil {
			return AcquireResult{}, err
		}

		res, taken, err := m.grantRandomFreeToken(ctx, cache, ownerID)
		if err != nil || !taken {
			return res, err
		}
		// Another owner holds the token. Try a different one.
	}
	return AcquireResult{Outcome: types.SemaphoreAcquireOutcomeNoSlot}, nil
}

// grantRandomFreeToken picks a random free token and writes its grant, holding the bucket lock for
// the pick, the write and the record. It answers NoSlot if no token is free. taken is true if
// another owner holds the token, so the caller can try another one.
func (m *semaphoreManagerImpl) grantRandomFreeToken(ctx context.Context, cache *bucketCache, ownerID string) (res AcquireResult, taken bool, err error) {
	if err := cache.lock(ctx); err != nil {
		return AcquireResult{}, false, err
	}
	defer cache.unlock()

	tokenID, ok := cache.pickFreeToken()
	if !ok {
		return AcquireResult{Outcome: types.SemaphoreAcquireOutcomeNoSlot}, false, nil
	}

	// This should not happen, since the cache only holds tokens in the range. It is checked anyway
	// because a grant on a token with no row creates the row, so a token outside the range would
	// let one more owner in than the semaphore's size allows.
	if !m.tokenRange.Contains(tokenID) {
		m.logger.Error("Semaphore grant picked a token this bucket does not own",
			tag.Dynamic("token-id", tokenID),
			tag.Dynamic("first-token", m.tokenRange.First),
			tag.Dynamic("last-token", m.tokenRange.Last()))
		return AcquireResult{}, false, &types.InternalServiceError{Message: fmt.Sprintf("token %d is outside the range of bucket %v", tokenID, m.id)}
	}

	resp, err := m.db.grantToken(ctx, tokenID, ownerID)
	if err != nil {
		// Nothing is recorded, so tokenID stays free. If the write did land, the database refuses
		// the next grant on it and the cache marks it unavailable.
		return AcquireResult{}, false, err
	}
	recordErr := cache.recordGrantResult(ownerID, tokenID, resp)

	switch resp.Outcome {
	case persistence.SemaphoreGrantApplied:
		return AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: tokenID}, false, nil

	case persistence.SemaphoreGrantSlotTaken:
		return AcquireResult{}, true, nil

	case persistence.SemaphoreGrantAlreadyHeld:
		// The owner already holds a token, so answer with it.
		if recordErr != nil {
			// The owner row names a token this bucket does not own, or no token at all, so it is
			// corrupt. Answering Acquired would hand out a token outside the bucket. Every acquire
			// for this owner fails this way until the owner row is fixed.
			return AcquireResult{}, false, &types.InternalServiceError{Message: fmt.Sprintf("grant reported AlreadyHeld with an invalid token for bucket %v: %v", m.id, recordErr)}
		}
		return AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: resp.HeldToken}, false, nil

	default:
		// Unreachable through the nosql store, which rejects unknown outcomes itself, but that is
		// one store's guarantee, not the interface's, so check anyway.
		return AcquireResult{}, false, &types.InternalServiceError{Message: fmt.Sprintf("unexpected grant outcome %v for bucket %v", resp.Outcome, m.id)}
	}
}
