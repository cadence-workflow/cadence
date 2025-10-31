package cache

import (
	"context"
	"errors"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

// This implements a generic host-scoped budget manager suitable for
// both non-evicting caches (admission/backpressure) and evicting caches (e.g., LRU).
//
// Fair Share Logic:
// The budget manager implements a two-tier soft cap system to fairly distribute capacity across
// multiple active caches while preventing any single cache from monopolizing resources:
//   • Free space tier: (threshold * capacity) - shared by all caches on a first-come basis
//   • Fair share tier: ((1 - threshold) * capacity) / activeCaches - allocated per active cache
//
// An active cache is one that currently has non-zero usage (usedBytes > 0 or usedCount > 0).
// This ensures that when multiple caches are active, each gets a guaranteed fair share of capacity,
// preventing scenarios where one cache consumes all available space and starves others.
//
// There are two admission modes available:
//   • AdmissionOptimistic => add-then-undo on failure; may briefly overshoot limits
//   • AdmissionStrict     => pre-check with CAS; never overshoots limits
//
// Note: Soft cap admission is always optimistic regardless of the configured mode.
//
// There are two modes available for reclaiming memory from the cache:
//   • ReserveOrReclaimSelfRelease    => reclaim does its own Release() calls (per-item or per-batch).
//   • ReserveOrReclaimManagerRelease => reclaim returns totals; manager calls Release() once.
//
// Pick the reclaim mode that fits the cache's eviction style (self-release vs manager-release)
// and admission mode based on whether temporary overshooting is acceptable (Optimistic vs Strict).
//
// How to use it:
//
// Manager is a means to track cache size across multiple caches at the Module or Host level.
// It solves the problem of accounting memory for multiple caches working with a limited total amount of memory
// in a centralized way, allowing each cache to operate independently while respecting global limits.
//
// The cache implementation should use the callback-based methods to ensure proper budget tracking
// with automatic cleanup on errors.
//
// Example usage - Adding an item to the cache:
//
//	func (c *myCache) Put(key, value interface{}) error {
//		itemSizeBytes := calculateSize(value)
//		return c.budgetMgr.ReserveWithCallback(c.cacheID, itemSizeBytes, 1, func() error {
//			return c.cache.Put(key, value)
//		})
//	}
//
// Example usage - Removing an item from the cache:
//
//	func (c *myCache) Delete(key interface{}) error {
//		return c.budgetMgr.ReleaseWithCallback(c.cacheID, func() (uint64, int64, error) {
//			bytes, count, err := c.cache.Delete(key)
//			return bytes, count, err
//		})
//	}
//
// Example usage - Adding with automatic eviction (manager-release pattern):
//
//	func (c *myCache) PutWithEviction(ctx context.Context, key, value interface{}) error {
//		itemSizeBytes := calculateSize(value)
//		return c.budgetMgr.ReserveOrReclaimManagerReleaseWithCallback(
//			ctx, c.cacheID, itemSizeBytes, 1, true,
//			func(needBytes uint64, needCount int64) (uint64, int64) {
//				return c.cache.EvictLRU(needBytes, needCount)
//			},
//			func() error {
//				return c.cache.Put(key, value)
//			},
//		)
//	}
//
// Example usage - Adding with automatic eviction (self-release pattern):
//
//	func (c *myCache) PutWithEviction(ctx context.Context, key, value interface{}) error {
//		itemSizeBytes := calculateSize(value)
//		return c.budgetMgr.ReserveOrReclaimSelfReleaseWithCallback(
//			ctx, c.cacheID, itemSizeBytes, 1, true,
//			func(needBytes uint64, needCount int64) {
//				// Evict items one-by-one until we've freed enough
//				var freedBytes, freedCount uint64, int64
//				for freedBytes < needBytes || freedCount < needCount {
//					evictedBytes, evictedCount, err := c.cache.EvictOldest()
//					if err != nil {
//						break
//					}
//					c.budgetMgr.ReleaseForCache(c.cacheID, evictedBytes, evictedCount)
//					freedBytes += evictedBytes
//					freedCount += evictedCount
//				}
//			},
//			func() error {
//				return c.cache.Put(key, value)
//			},
//		)
//	}

type AdmissionMode uint8

const (
	AdmissionOptimistic AdmissionMode = iota // add-then-undo; may overshoot briefly
	AdmissionStrict                          // CAS pre-check; never overshoots
)

const (
	budgetTypeBytes = "bytes"
	budgetTypeCount = "count"
)

var (
	ErrBytesBudgetExceeded        = errors.New("bytes budget exceeded")
	ErrCountBudgetExceeded        = errors.New("count budget exceeded")
	ErrBytesSoftCapExceeded       = errors.New("bytes soft cap threshold exceeded")
	ErrCountSoftCapExceeded       = errors.New("count soft cap threshold exceeded")
	ErrOverflow                   = errors.New("budget counter overflow")
	ErrInvalidValue               = errors.New("invalid negative reserve value")
	ErrInsufficientUsageToReclaim = errors.New("insufficient usage to reclaim")
)

type Manager interface {
	// Callback-based reserve methods (reserve -> callback -> release on error)

	// ReserveWithCallback reserves capacity, executes callback, releases on callback error.
	ReserveWithCallback(cacheID string, nBytes uint64, nCount int64, callback func() error) error
	// ReserveBytesWithCallback reserves bytes, executes callback, releases on callback error.
	ReserveBytesWithCallback(cacheID string, nBytes uint64, callback func() error) error
	// ReserveCountWithCallback reserves count, executes callback, releases on callback error.
	ReserveCountWithCallback(cacheID string, nCount int64, callback func() error) error

	// Callback-based release methods (callback -> release on success)

	// ReleaseWithCallback executes callback, releases capacity if callback succeeds.
	ReleaseWithCallback(cacheID string, callback func() (nBytes uint64, nCount int64, err error)) error
	// ReleaseBytesWithCallback executes callback, releases bytes if callback succeeds.
	ReleaseBytesWithCallback(cacheID string, callback func() (freedBytes uint64, err error)) error
	// ReleaseCountWithCallback executes callback, releases count if callback succeeds.
	ReleaseCountWithCallback(cacheID string, callback func() (freedCount int64, err error)) error

	// Callback-based reclaim methods

	// ReserveOrReclaimSelfReleaseWithCallback reserves/reclaims capacity, executes callback, releases on callback error.
	ReserveOrReclaimSelfReleaseWithCallback(ctx context.Context, cacheID string, nBytes uint64, nCount int64, retriable bool, reclaim ReclaimSelfRelease, callback func() error) error

	// ReserveOrReclaimManagerReleaseWithCallback reserves/reclaims capacity, executes callback, releases on callback error.
	ReserveOrReclaimManagerReleaseWithCallback(ctx context.Context, cacheID string, nBytes uint64, nCount int64, retriable bool, reclaim ReclaimManagerRelease, callback func() error) error

	// UsedBytes returns current used bytes.
	UsedBytes() uint64
	// UsedCount returns current used count.
	UsedCount() int64
	// CapacityBytes returns the current effective bytes capacity (math.MaxUint64 means "unlimited").
	CapacityBytes() uint64
	// CapacityCount returns the current effective count capacity (math.MaxInt64 means "unlimited").
	CapacityCount() int64
}

// ReclaimSelfRelease is used when the caller will call mgr.Release(...) per item or per batch.
type ReclaimSelfRelease func(needBytes uint64, needCount int64)

// ReclaimManagerRelease is used when the manager should call Release once with totals.
// IMPORTANT: Do NOT call mgr.Release(...) inside this callback, or you will double-release.
type ReclaimManagerRelease func(needBytes uint64, needCount int64) (freedBytes uint64, freedCount int64)

// CapEnforcementResult contains the result of capacity enforcement (both hard and soft cap) for a cache
type CapEnforcementResult struct {
	Error          error  // nil if allowed, specific error if not allowed
	FreeBytes      uint64 // bytes allocated from free capacity (on success only)
	FairShareBytes uint64 // bytes allocated from fair share capacity (on success only)
	FreeCount      int64  // count allocated from free capacity (on success only)
	FairShareCount int64  // count allocated from fair share capacity (on success only)
	AvailableBytes uint64 // total bytes available for this cache (min of soft cap and hard cap constraints)
	AvailableCount int64  // total count available for this cache (min of soft cap and hard cap constraints)
}

// Usage tracks usage for a specific cache
type Usage struct {
	mu sync.Mutex // protects all fields below

	usedBytes uint64 // total bytes used by this cache
	usedCount int64  // total count used by this cache

	// Capacity type breakdown for fairness strategies
	fairShareCapacityBytes uint64 // bytes from (1-threshold) portion
	freeCapacityBytes      uint64 // bytes from threshold portion
	fairShareCapacityCount int64  // count from (1-threshold) portion
	freeCapacityCount      int64  // count from threshold portion
}

type manager struct {
	name                        string
	maxBytes                    dynamicproperties.IntPropertyFn
	maxCount                    dynamicproperties.IntPropertyFn
	reclaimBackoff              time.Duration
	admission                   AdmissionMode
	scope                       metrics.Scope
	logger                      log.Logger
	enforcementSoftCapThreshold dynamicproperties.FloatPropertyFn

	usedBytes uint64 // atomic - total usage across all caches
	usedCount int64  // atomic - total usage across all caches

	// Per-cache usage tracking using sync.Map for lock-free operations
	cacheUsage sync.Map // map[string]*Usage

	// Optimistic tracking of active caches (usage > 0)
	activeCacheCount int64 // atomic
}

// NewBudgetManager creates a new Manager.
//
// Parameters:
//   - name: manager name for identification
//   - maxBytes: bytes capacity semantics:
//     < 0  => enforcement disabled (unlimited; still tracked)
//     == 0 => caching fully disabled (deny all reserves)
//     > 0  => enforcement enabled with that capacity
//   - maxCount: count capacity semantics (same as maxBytes)
//   - admission: AdmissionOptimistic (add-then-undo) or AdmissionStrict (CAS pre-check)
//   - reclaimBackoff: optional sleep between reclaim attempts (defaults to ~100µs if zero)
//   - scope: metrics scope for emitting metrics (can be nil)
//   - logger: logger for diagnostic messages (can be nil)
//   - softCapThreshold: optional percentage (0.0-1.0) defining how capacity is split between two tiers:
//   - Free space tier: (threshold * capacity) - shared by all caches
//   - Fair share tier: ((1 - threshold) * capacity) / activeCaches - allocated per cache
//     The fair share capacity is equal to the fair share capacity ((1 - threshold) * capacity) divided by the number
//     of active caches (caches with usage > 0).
func NewBudgetManager(
	name string,
	maxBytes dynamicproperties.IntPropertyFn,
	maxCount dynamicproperties.IntPropertyFn,
	admission AdmissionMode,
	reclaimBackoff time.Duration,
	scope metrics.Scope,
	logger log.Logger,
	softCapThreshold dynamicproperties.FloatPropertyFn, // nil defaults to 1.0 (disabled)
) Manager {
	if scope == nil {
		scope = metrics.NoopScope
	}
	if maxBytes == nil {
		maxBytes = dynamicproperties.GetIntPropertyFn(-1) // unlimited if maxBytes is not provided
	}
	if maxCount == nil {
		maxCount = dynamicproperties.GetIntPropertyFn(-1) // unlimited if maxCount is not provided
	}
	if softCapThreshold == nil {
		softCapThreshold = dynamicproperties.GetFloatPropertyFn(1.0) // no threshold if softCapThreshold is not provided
	}
	if admission == 0 {
		admission = AdmissionOptimistic
	}

	mgr := &manager{
		name:                        name,
		maxBytes:                    maxBytes,
		maxCount:                    maxCount,
		reclaimBackoff:              reclaimBackoff,
		admission:                   admission,
		scope:                       scope.Tagged(metrics.BudgetManagerNameTag(name)),
		logger:                      logger,
		enforcementSoftCapThreshold: softCapThreshold,
	}

	// Emit initial metrics
	mgr.updateMetrics()

	return mgr
}

// updateMetrics emits current state metrics
func (m *manager) updateMetrics() {
	// Emit capacity metrics
	capacityBytes := m.CapacityBytes()
	if capacityBytes != math.MaxUint64 {
		m.scope.UpdateGauge(metrics.BudgetManagerCapacityBytes, float64(capacityBytes))
	}

	capacityCount := m.CapacityCount()
	if capacityCount != math.MaxInt64 {
		m.scope.UpdateGauge(metrics.BudgetManagerCapacityCount, float64(capacityCount))
	}

	// Emit usage metrics
	m.scope.UpdateGauge(metrics.BudgetManagerUsedBytes, float64(m.UsedBytes()))
	m.scope.UpdateGauge(metrics.BudgetManagerUsedCount, float64(m.UsedCount()))

	// Emit soft threshold
	m.scope.UpdateGauge(metrics.BudgetManagerSoftThreshold, m.enforcementSoftCapThreshold())

	// Emit active cache count
	m.scope.UpdateGauge(metrics.BudgetManagerActiveCacheCount, float64(atomic.LoadInt64(&m.activeCacheCount)))
}

// emitHardCapExceeded logs when hard capacity limit is exceeded and increments counter
func (m *manager) emitHardCapExceeded(cacheID, budgetType string, requested, available uint64) {
	if m.scope != nil {
		m.scope.IncCounter(metrics.BudgetManagerHardCapExceeded)
	}

	if m.logger != nil {
		m.logger.Debug("Hard capacity limit exceeded",
			tag.Name(m.name),
			tag.Dynamic("cache_id", cacheID),
			tag.Value(budgetType),
			tag.Counter(int(requested)),
			tag.Number(int64(available)),
		)
	}
}

// emitSoftCapExceeded logs when soft cap is exceeded and increments counter
func (m *manager) emitSoftCapExceeded(cacheID, budgetType string, requested, available uint64) {
	if m.scope != nil {
		m.scope.IncCounter(metrics.BudgetManagerSoftCapExceeded)
	}

	if m.logger != nil {
		m.logger.Warn("Soft capacity limit exceeded",
			tag.Name(m.name),
			tag.Dynamic("cache_id", cacheID),
			tag.Value(budgetType),
			tag.Counter(int(requested)),
			tag.Number(int64(available)),
		)
	}
}

// emitCapacityExceeded emits appropriate metrics based on the error type
func (m *manager) emitCapacityExceeded(cacheID string, err error, requestedBytes uint64, requestedCount int64, capResult CapEnforcementResult) {
	switch err {
	case ErrBytesBudgetExceeded:
		m.emitHardCapExceeded(cacheID, budgetTypeBytes, requestedBytes, capResult.AvailableBytes)
	case ErrCountBudgetExceeded:
		m.emitHardCapExceeded(cacheID, budgetTypeCount, uint64(requestedCount), uint64(capResult.AvailableCount))
	case ErrBytesSoftCapExceeded:
		m.emitSoftCapExceeded(cacheID, budgetTypeBytes, requestedBytes, capResult.AvailableBytes)
	case ErrCountSoftCapExceeded:
		m.emitSoftCapExceeded(cacheID, budgetTypeCount, uint64(requestedCount), uint64(capResult.AvailableCount))
	}
}

// Cache-aware Reserve methods implementing two-tier soft cap logic

func (m *manager) ReserveForCache(cacheID string, nBytes uint64, nCount int64) error {
	// Check capacity constraints (both hard and soft cap)
	capResult := m.enforceCapForCache(cacheID, nBytes, nCount)
	if capResult.Error != nil {
		m.emitCapacityExceeded(cacheID, capResult.Error, nBytes, nCount, capResult)
		return capResult.Error
	}

	cacheUsage := m.getCacheUsage(cacheID)

	cacheUsage.mu.Lock()
	wasActive := cacheUsage.usedBytes > 0 || cacheUsage.usedCount > 0

	if nBytes > 0 {
		if err := m.reserveBytes(nBytes); err != nil {
			cacheUsage.mu.Unlock()
			return err
		}
		cacheUsage.usedBytes += nBytes
		cacheUsage.fairShareCapacityBytes += capResult.FairShareBytes
		cacheUsage.freeCapacityBytes += capResult.FreeBytes
	}
	if nCount > 0 {
		if err := m.reserveCount(nCount); err != nil {
			if nBytes > 0 {
				m.releaseBytes(nBytes)
				cacheUsage.usedBytes -= nBytes
				cacheUsage.fairShareCapacityBytes -= capResult.FairShareBytes
				cacheUsage.freeCapacityBytes -= capResult.FreeBytes
			}
			cacheUsage.mu.Unlock()
			return err
		}
		cacheUsage.usedCount += nCount
		cacheUsage.fairShareCapacityCount += capResult.FairShareCount
		cacheUsage.freeCapacityCount += capResult.FreeCount
	}

	cacheUsage.mu.Unlock()

	// Update active cache count after all operations succeed
	m.updateActiveCacheCount(cacheUsage, wasActive)

	// Update metrics after successful reservation
	m.updateMetrics()

	return nil
}

func (m *manager) ReserveBytesForCache(cacheID string, nBytes uint64) error {
	// Check capacity constraints (both hard and soft cap)
	capResult := m.enforceCapForCache(cacheID, nBytes, 0)
	if capResult.Error != nil {
		m.emitCapacityExceeded(cacheID, capResult.Error, nBytes, 0, capResult)
		return capResult.Error
	}

	cacheUsage := m.getCacheUsage(cacheID)

	cacheUsage.mu.Lock()
	wasActive := cacheUsage.usedBytes > 0 || cacheUsage.usedCount > 0

	if err := m.reserveBytes(nBytes); err != nil {
		cacheUsage.mu.Unlock()
		return err
	}

	cacheUsage.usedBytes += nBytes
	cacheUsage.fairShareCapacityBytes += capResult.FairShareBytes
	cacheUsage.freeCapacityBytes += capResult.FreeBytes
	cacheUsage.mu.Unlock()

	m.updateActiveCacheCount(cacheUsage, wasActive)

	// Update metrics after successful reservation
	m.updateMetrics()

	return nil
}

func (m *manager) ReserveCountForCache(cacheID string, nCount int64) error {
	if nCount < 0 {
		if m.logger != nil {
			m.logger.Error("Invalid negative count value in ReserveCountForCache",
				tag.Dynamic("cache_id", cacheID),
				tag.Key("requested"), tag.Value(nCount),
			)
		}
		return ErrInvalidValue
	}
	// Check capacity constraints (both hard and soft cap)
	capResult := m.enforceCapForCache(cacheID, 0, nCount)
	if capResult.Error != nil {
		m.emitCapacityExceeded(cacheID, capResult.Error, 0, nCount, capResult)
		return capResult.Error
	}

	cacheUsage := m.getCacheUsage(cacheID)

	cacheUsage.mu.Lock()
	wasActive := cacheUsage.usedBytes > 0 || cacheUsage.usedCount > 0

	if err := m.reserveCount(nCount); err != nil {
		cacheUsage.mu.Unlock()
		return err
	}

	cacheUsage.usedCount += nCount
	cacheUsage.fairShareCapacityCount += capResult.FairShareCount
	cacheUsage.freeCapacityCount += capResult.FreeCount
	cacheUsage.mu.Unlock()

	m.updateActiveCacheCount(cacheUsage, wasActive)

	// Update metrics after successful reservation
	m.updateMetrics()

	return nil
}

func (m *manager) reserveBytes(n uint64) error {
	switch m.admission {
	case AdmissionStrict:
		return m.reserveBytesStrict(n)
	default:
		return m.reserveBytesOptimistic(n)
	}
}

func (m *manager) reserveCount(n int64) error {
	switch m.admission {
	case AdmissionStrict:
		return m.reserveCountStrict(n)
	default:
		return m.reserveCountOptimistic(n)
	}
}

func (m *manager) reserveBytesStrict(n uint64) error {
	capB := m.CapacityBytes()
	for {
		old := atomic.LoadUint64(&m.usedBytes)

		if old > math.MaxUint64-n {
			if m.logger != nil {
				m.logger.Error("Bytes budget overflow detected in strict mode",
					tag.Key("requested"), tag.Value(n),
					tag.Key("current-used"), tag.Value(old),
				)
			}
			return ErrOverflow
		}
		if old+n > capB {
			m.emitHardCapExceeded("", budgetTypeBytes, old+n, capB)
			return ErrBytesBudgetExceeded
		}
		if atomic.CompareAndSwapUint64(&m.usedBytes, old, old+n) {
			return nil
		}
		// retry on contention
	}
}

func (m *manager) reserveBytesOptimistic(n uint64) error {
	capB := m.CapacityBytes()
	newVal := atomic.AddUint64(&m.usedBytes, n)
	if newVal < n { // wrap-around
		m.releaseBytes(n)
		if m.logger != nil {
			m.logger.Error("Bytes budget overflow detected",
				tag.Key("requested"), tag.Value(n),
				tag.Key("previous-used"), tag.Value(newVal-n),
			)
		}
		return ErrOverflow
	}
	if newVal <= capB {
		return nil
	}
	m.releaseBytes(n)
	m.emitHardCapExceeded("", budgetTypeBytes, newVal, capB)
	return ErrBytesBudgetExceeded
}

func (m *manager) reserveCountStrict(n int64) error {
	capC := m.CapacityCount()
	for {
		old := atomic.LoadInt64(&m.usedCount)

		if old > math.MaxInt64-n {
			if m.logger != nil {
				m.logger.Error("Count budget overflow detected in strict mode",
					tag.Key("requested"), tag.Value(n),
					tag.Key("current-used"), tag.Value(old),
				)
			}
			return ErrOverflow
		}
		if old+n > capC {
			m.emitHardCapExceeded("", budgetTypeCount, uint64(old+n), uint64(capC))
			return ErrCountBudgetExceeded
		}
		if atomic.CompareAndSwapInt64(&m.usedCount, old, old+n) {
			return nil
		}
		// retry on contention
	}
}

func (m *manager) reserveCountOptimistic(n int64) error {
	capC := m.CapacityCount()
	newVal := atomic.AddInt64(&m.usedCount, n)
	if newVal < 0 { // wrap-around
		m.releaseCount(n)
		return ErrOverflow
	}
	if newVal <= capC {
		return nil
	}
	m.releaseCount(n)
	m.emitHardCapExceeded("", budgetTypeCount, uint64(newVal), uint64(capC))
	return ErrCountBudgetExceeded
}

func (m *manager) ReserveOrReclaimSelfRelease(
	ctx context.Context,
	cacheID string,
	nBytes uint64,
	nCount int64,
	retriable bool,
	reclaim ReclaimSelfRelease,
) error {
	if err := m.ReserveForCache(cacheID, nBytes, nCount); err == nil {
		return nil
	} else if !retriable {
		return err // single-shot; no reclaim
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var needB uint64
		var needC int64

		// Check capacity constraints (both soft and hard cap)
		capResult := m.enforceCapForCache(cacheID, nBytes, nCount)
		if capResult.Error == nil {
			// Capacity available - try to reserve
			if err := m.ReserveForCache(cacheID, nBytes, nCount); err == nil {
				return nil
			}
			// lost the race; loop
			continue
		}

		// Calculate reclaim amounts based on available capacity
		allowedBytes := capResult.AvailableBytes
		allowedCount := capResult.AvailableCount
		needB = nBytes - allowedBytes
		needC = nCount - allowedCount

		// Check if this cache can reclaim enough to satisfy the constraint
		cacheUsage := m.getCacheUsage(cacheID)
		cacheUsage.mu.Lock()
		currentCacheBytes := cacheUsage.usedBytes
		currentCacheCount := cacheUsage.usedCount

		if (currentCacheBytes+allowedBytes < nBytes) || (currentCacheCount+allowedCount < nCount) {
			cacheUsage.mu.Unlock()
			// Cache doesn't have enough to reclaim
			return ErrInsufficientUsageToReclaim
		}
		cacheUsage.mu.Unlock()

		if reclaim != nil && (needB > 0 || needC > 0) {
			// The manager uses only atomic operations (no locks), so the cache
			// can safely take its own locks (e.g., for protecting internal data structures
			// during eviction) without risk of deadlock. The cache must call ReleaseForCache
			// separately after evicting items.
			reclaim(needB, needC)

			// Fast path: try to consume immediately after reclaim before yielding.
			if err := m.ReserveForCache(cacheID, nBytes, nCount); err == nil {
				return nil
			}
		}

		m.yield() // tiny backoff to avoid busy spin
	}
}

func (m *manager) ReserveOrReclaimManagerRelease(
	ctx context.Context,
	cacheID string,
	nBytes uint64,
	nCount int64,
	retriable bool,
	reclaim ReclaimManagerRelease,
) error {
	if err := m.ReserveForCache(cacheID, nBytes, nCount); err == nil {
		return nil
	} else if !retriable {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err() // propagate context cancellation/deadline
		default:
		}

		var needB uint64
		var needC int64

		// Check capacity constraints (both soft and hard cap)
		capResult := m.enforceCapForCache(cacheID, nBytes, nCount)
		if capResult.Error == nil {
			// Capacity available - try to reserve
			if err := m.ReserveForCache(cacheID, nBytes, nCount); err == nil {
				return nil
			}
			// lost the race; loop
			continue
		}

		// Calculate reclaim amounts based on available capacity
		allowedBytes := capResult.AvailableBytes
		allowedCount := capResult.AvailableCount
		needB = nBytes - allowedBytes
		needC = nCount - allowedCount

		// Check if this cache can reclaim enough to satisfy the constraint
		cacheUsage := m.getCacheUsage(cacheID)
		cacheUsage.mu.Lock()
		currentCacheBytes := cacheUsage.usedBytes
		currentCacheCount := cacheUsage.usedCount

		if (currentCacheBytes+allowedBytes < nBytes) || (currentCacheCount+allowedCount < nCount) {
			cacheUsage.mu.Unlock()
			// Cache doesn't have enough to reclaim
			return ErrInsufficientUsageToReclaim
		}
		cacheUsage.mu.Unlock()

		if reclaim != nil && (needB > 0 || needC > 0) {
			fb, fc := reclaim(needB, needC)
			if fb > 0 || fc > 0 {
				m.ReleaseForCache(cacheID, fb, fc)

				// Fast-path: try to admit immediately after releasing
				if err := m.ReserveForCache(cacheID, nBytes, nCount); err == nil {
					return nil
				}
			}
		}

		m.yield() // small backoff to avoid busy spin
	}
}

func (m *manager) yield() {
	runtime.Gosched()
	if m.reclaimBackoff > 0 {
		time.Sleep(m.reclaimBackoff)
		return
	}
	time.Sleep(100 * time.Microsecond)
}

// Cache-aware Release methods

func (m *manager) ReleaseForCache(cacheID string, nBytes uint64, nCount int64) {
	cacheUsage := m.getCacheUsage(cacheID)

	cacheUsage.mu.Lock()
	wasActive := cacheUsage.usedBytes > 0 || cacheUsage.usedCount > 0

	if nBytes > 0 {
		m.releaseBytes(nBytes)
		// Update cache-specific usage tracking with over-release protection
		if nBytes > cacheUsage.usedBytes {
			if m.logger != nil {
				m.logger.Warn("Cache bytes over-release detected",
					tag.Key("cache_id"), tag.Value(cacheID),
					tag.Key("requested-release"), tag.Value(nBytes),
					tag.Key("current-used"), tag.Value(cacheUsage.usedBytes),
				)
			}
			cacheUsage.usedBytes = 0
			cacheUsage.fairShareCapacityBytes = 0
			cacheUsage.freeCapacityBytes = 0
		} else {
			cacheUsage.usedBytes -= nBytes
			// Update capacity type breakdown - subtract from fairShare first, then free
			remainingBytes := nBytes
			fairShareToSubtract := min(remainingBytes, cacheUsage.fairShareCapacityBytes)
			if fairShareToSubtract > 0 {
				cacheUsage.fairShareCapacityBytes -= fairShareToSubtract
				remainingBytes -= fairShareToSubtract
			}
			if remainingBytes > 0 {
				cacheUsage.freeCapacityBytes -= remainingBytes
			}
		}
	}
	if nCount > 0 {
		m.releaseCount(nCount)
		// Update cache-specific usage tracking with over-release protection
		if nCount > cacheUsage.usedCount {
			if m.logger != nil {
				m.logger.Warn("Cache count over-release detected",
					tag.Key("cache_id"), tag.Value(cacheID),
					tag.Key("requested-release"), tag.Value(nCount),
					tag.Key("current-used"), tag.Value(cacheUsage.usedCount),
				)
			}
			cacheUsage.usedCount = 0
			cacheUsage.fairShareCapacityCount = 0
			cacheUsage.freeCapacityCount = 0
		} else {
			cacheUsage.usedCount -= nCount
			// Update capacity type breakdown - subtract from fairShare first, then free
			remainingCount := nCount
			fairShareCountToSubtract := min(remainingCount, cacheUsage.fairShareCapacityCount)
			if fairShareCountToSubtract > 0 {
				cacheUsage.fairShareCapacityCount -= fairShareCountToSubtract
				remainingCount -= fairShareCountToSubtract
			}
			if remainingCount > 0 {
				cacheUsage.freeCapacityCount -= remainingCount
			}
		}
	}
	cacheUsage.mu.Unlock()

	// Update active cache count after all operations
	m.updateActiveCacheCount(cacheUsage, wasActive)

	// Update metrics after successful release
	m.updateMetrics()
}

func (m *manager) ReleaseBytesForCache(cacheID string, n uint64) {
	m.releaseBytes(n)
	// Update cache-specific usage tracking
	cacheUsage := m.getCacheUsage(cacheID)

	cacheUsage.mu.Lock()
	wasActive := cacheUsage.usedBytes > 0 || cacheUsage.usedCount > 0
	// Update cache-specific usage tracking with over-release protection
	if n > cacheUsage.usedBytes {
		if m.logger != nil {
			m.logger.Warn("Cache bytes over-release detected",
				tag.Key("cache_id"), tag.Value(cacheID),
				tag.Key("requested-release"), tag.Value(n),
				tag.Key("current-used"), tag.Value(cacheUsage.usedBytes),
			)
		}
		cacheUsage.usedBytes = 0
		cacheUsage.fairShareCapacityBytes = 0
		cacheUsage.freeCapacityBytes = 0
	} else {
		cacheUsage.usedBytes -= n
		// Update capacity type breakdown - subtract from fairShare first, then free
		remainingBytes := n
		fairShareToSubtract := min(remainingBytes, cacheUsage.fairShareCapacityBytes)
		if fairShareToSubtract > 0 {
			cacheUsage.fairShareCapacityBytes -= fairShareToSubtract
			remainingBytes -= fairShareToSubtract
		}
		if remainingBytes > 0 {
			cacheUsage.freeCapacityBytes -= remainingBytes
		}
	}
	cacheUsage.mu.Unlock()

	m.updateActiveCacheCount(cacheUsage, wasActive)

	// Update metrics after successful release
	m.updateMetrics()
}

func (m *manager) ReleaseCountForCache(cacheID string, n int64) {
	m.releaseCount(n)
	// Update cache-specific usage tracking
	cacheUsage := m.getCacheUsage(cacheID)

	cacheUsage.mu.Lock()
	wasActive := cacheUsage.usedBytes > 0 || cacheUsage.usedCount > 0
	// Update cache-specific usage tracking with over-release protection
	if n > cacheUsage.usedCount {
		if m.logger != nil {
			m.logger.Warn("Cache count over-release detected",
				tag.Key("cache_id"), tag.Value(cacheID),
				tag.Key("requested-release"), tag.Value(n),
				tag.Key("current-used"), tag.Value(cacheUsage.usedCount),
			)
		}
		cacheUsage.usedCount = 0
		cacheUsage.fairShareCapacityCount = 0
		cacheUsage.freeCapacityCount = 0
	} else {
		cacheUsage.usedCount -= n
		// Update capacity type breakdown - subtract from fairShare first, then free
		remainingCount := n
		fairShareCountToSubtract := min(remainingCount, cacheUsage.fairShareCapacityCount)
		if fairShareCountToSubtract > 0 {
			cacheUsage.fairShareCapacityCount -= fairShareCountToSubtract
			remainingCount -= fairShareCountToSubtract
		}
		if remainingCount > 0 {
			cacheUsage.freeCapacityCount -= remainingCount
		}
	}
	cacheUsage.mu.Unlock()

	m.updateActiveCacheCount(cacheUsage, wasActive)

	// Update metrics after successful release
	m.updateMetrics()
}

func (m *manager) releaseBytes(n uint64) {
	for {
		old := atomic.LoadUint64(&m.usedBytes)
		var newVal uint64
		if n > old {
			// Over-release detected - clamp to 0 to prevent uint64 wraparound
			newVal = 0
			if atomic.CompareAndSwapUint64(&m.usedBytes, old, newVal) {
				if m.logger != nil {
					m.logger.Warn("Bytes over-release detected",
						tag.Key("requested-release"), tag.Value(n),
						tag.Key("current-used"), tag.Value(old),
					)
				}
				return
			}
		} else {
			newVal = old - n
			if atomic.CompareAndSwapUint64(&m.usedBytes, old, newVal) {
				return
			}
		}
		// retry on contention
	}
}

func (m *manager) releaseCount(n int64) {
	if n <= 0 {
		return
	}
	for {
		old := atomic.LoadInt64(&m.usedCount)
		var newVal int64
		if n > old {
			// Over-release detected - clamp to 0
			newVal = 0
			if atomic.CompareAndSwapInt64(&m.usedCount, old, newVal) {
				if m.logger != nil {
					m.logger.Warn("Count over-release detected",
						tag.Key("requested-release"), tag.Value(n),
						tag.Key("current-used"), tag.Value(old),
					)
				}
				return
			}
		} else {
			newVal = old - n
			if atomic.CompareAndSwapInt64(&m.usedCount, old, newVal) {
				return
			}
		}
		// retry on contention
	}
}

func (m *manager) ReserveWithCallback(cacheID string, nBytes uint64, nCount int64, callback func() error) error {
	if err := m.ReserveForCache(cacheID, nBytes, nCount); err != nil {
		return err
	}
	if err := callback(); err != nil {
		m.ReleaseForCache(cacheID, nBytes, nCount)
		return err
	}
	return nil
}

func (m *manager) ReserveBytesWithCallback(cacheID string, nBytes uint64, callback func() error) error {
	if err := m.ReserveBytesForCache(cacheID, nBytes); err != nil {
		return err
	}
	if err := callback(); err != nil {
		m.ReleaseBytesForCache(cacheID, nBytes)
		return err
	}
	return nil
}

func (m *manager) ReserveCountWithCallback(cacheID string, nCount int64, callback func() error) error {
	if err := m.ReserveCountForCache(cacheID, nCount); err != nil {
		return err
	}
	if err := callback(); err != nil {
		m.ReleaseCountForCache(cacheID, nCount)
		return err
	}
	return nil
}

func (m *manager) ReleaseWithCallback(cacheID string, callback func() (freedBytes uint64, freedCount int64, err error)) error {
	freedBytes, freedCount, err := callback()
	if err != nil {
		return err
	}
	m.ReleaseForCache(cacheID, freedBytes, freedCount)
	return nil
}

func (m *manager) ReleaseBytesWithCallback(cacheID string, callback func() (freedBytes uint64, err error)) error {
	freedBytes, err := callback()
	if err != nil {
		return err
	}
	m.ReleaseBytesForCache(cacheID, freedBytes)
	return nil
}

func (m *manager) ReleaseCountWithCallback(cacheID string, callback func() (freedCount int64, err error)) error {
	freedCount, err := callback()
	if err != nil {
		return err
	}
	m.ReleaseCountForCache(cacheID, freedCount)
	return nil
}

func (m *manager) ReserveOrReclaimSelfReleaseWithCallback(ctx context.Context, cacheID string, nBytes uint64, nCount int64, retriable bool, reclaim ReclaimSelfRelease, callback func() error) error {
	if err := m.ReserveOrReclaimSelfRelease(ctx, cacheID, nBytes, nCount, retriable, reclaim); err != nil {
		return err
	}
	if err := callback(); err != nil {
		m.ReleaseForCache(cacheID, nBytes, nCount)
		return err
	}
	return nil
}

func (m *manager) ReserveOrReclaimManagerReleaseWithCallback(ctx context.Context, cacheID string, nBytes uint64, nCount int64, retriable bool, reclaim ReclaimManagerRelease, callback func() error) error {
	if err := m.ReserveOrReclaimManagerRelease(ctx, cacheID, nBytes, nCount, retriable, reclaim); err != nil {
		return err
	}
	if err := callback(); err != nil {
		m.ReleaseForCache(cacheID, nBytes, nCount)
		return err
	}
	return nil
}

func (m *manager) UsedBytes() uint64 { return atomic.LoadUint64(&m.usedBytes) }
func (m *manager) UsedCount() int64  { return atomic.LoadInt64(&m.usedCount) }

func (m *manager) CapacityBytes() uint64 {
	v := m.maxBytes()
	if v < 0 {
		return math.MaxUint64
	}
	return uint64(v) // includes 0 as a valid, enforced cap
}

func (m *manager) CapacityCount() int64 {
	v := m.maxCount()
	if v < 0 {
		return math.MaxInt64
	}
	// clamp just in case
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v) // includes 0 as a valid, enforced cap
}

func (m *manager) AvailableBytes() uint64 {
	c := m.CapacityBytes()
	if c == 0 {
		return 0
	}
	u := m.UsedBytes()
	if u >= c {
		return 0
	}
	return c - u
}
func (m *manager) AvailableCount() int64 {
	c := m.CapacityCount()
	if c == 0 {
		return 0
	}
	u := m.UsedCount()
	if u >= c {
		return 0
	}
	return c - u
}

// getCacheUsage returns the CacheUsage for a given cacheID, creating one if it doesn't exist
func (m *manager) getCacheUsage(cacheID string) *Usage {
	if value, ok := m.cacheUsage.Load(cacheID); ok {
		return value.(*Usage)
	}

	// Create new cache usage entry
	newUsage := &Usage{}
	actual, _ := m.cacheUsage.LoadOrStore(cacheID, newUsage)
	return actual.(*Usage)
}

// getActiveCacheCount returns the current count of active caches
func (m *manager) getActiveCacheCount() int64 {
	return atomic.LoadInt64(&m.activeCacheCount)
}

// updateActiveCacheCount updates the active cache count when a cache transitions between active/inactive
func (m *manager) updateActiveCacheCount(cacheUsage *Usage, wasActive bool) {
	cacheUsage.mu.Lock()
	isActive := cacheUsage.usedBytes > 0 || cacheUsage.usedCount > 0
	cacheUsage.mu.Unlock()

	if !wasActive && isActive {
		atomic.AddInt64(&m.activeCacheCount, 1)
	} else if wasActive && !isActive {
		atomic.AddInt64(&m.activeCacheCount, -1)
	}
}

// enforceCapForCache implements both hard and soft cap logic
func (m *manager) enforceCapForCache(cacheID string, additionalBytes uint64, additionalCount int64) CapEnforcementResult {
	// Check if caching is completely disabled (zero capacity)
	if m.CapacityBytes() == 0 || m.CapacityCount() == 0 {
		var err error
		if m.CapacityBytes() == 0 {
			err = ErrBytesBudgetExceeded
		} else {
			err = ErrCountBudgetExceeded
		}
		return CapEnforcementResult{
			Error:          err,
			FreeBytes:      0,
			FairShareBytes: 0,
			FreeCount:      0,
			FairShareCount: 0,
			AvailableBytes: 0,
			AvailableCount: 0,
		}
	}

	// Get hard cap constraints
	hardCapAvailableBytes := m.AvailableBytes()
	hardCapAvailableCount := m.AvailableCount()

	// Check soft cap constraints
	managerThreshold := m.enforcementSoftCapThreshold()
	if managerThreshold >= 0 && managerThreshold < 1 {
		// Soft caps enabled - calculate soft cap constraints
		thresholdBytes := uint64(float64(m.CapacityBytes()) * managerThreshold)
		thresholdCount := int64(float64(m.CapacityCount()) * managerThreshold)

		// Calculate available free space
		freeSpaceBytes := uint64(0)
		if thresholdBytes > m.UsedBytes() {
			freeSpaceBytes = thresholdBytes - m.UsedBytes()
		}
		freeSpaceCount := int64(0)
		if thresholdCount > m.UsedCount() {
			freeSpaceCount = thresholdCount - m.UsedCount()
		}

		// Calculate per-cache fair share limits
		activeCaches := m.getActiveCacheCount()

		// Check if this cache is currently inactive but will become active
		cacheUsage := m.getCacheUsage(cacheID)
		cacheUsage.mu.Lock()
		cacheIsActive := cacheUsage.usedBytes > 0 || cacheUsage.usedCount > 0
		currentFairShareBytes := cacheUsage.fairShareCapacityBytes
		currentFairShareCount := cacheUsage.fairShareCapacityCount
		cacheUsage.mu.Unlock()

		if !cacheIsActive && (additionalBytes > 0 || additionalCount > 0) {
			// Cache will become active, include it in the count
			activeCaches++
		}

		fairShareCapacityBytes := uint64(float64(m.CapacityBytes()) * (1.0 - managerThreshold))
		fairShareCapacityCount := int64(float64(m.CapacityCount()) * (1.0 - managerThreshold))
		fairSharePerCacheBytes := fairShareCapacityBytes / uint64(activeCaches)
		fairSharePerCacheCount := fairShareCapacityCount / activeCaches

		// Calculate available fair share capacity for this cache
		fairShareAvailableBytes := uint64(0)
		if fairSharePerCacheBytes > currentFairShareBytes {
			fairShareAvailableBytes = fairSharePerCacheBytes - currentFairShareBytes
		}
		fairShareAvailableCount := int64(0)
		if fairSharePerCacheCount > currentFairShareCount {
			fairShareAvailableCount = fairSharePerCacheCount - currentFairShareCount
		}

		// Total soft cap available is free space + fair share for this cache
		softCapAvailableBytes := freeSpaceBytes + fairShareAvailableBytes
		softCapAvailableCount := freeSpaceCount + fairShareAvailableCount

		// Available capacity is minimum of soft cap and hard cap constraints
		availableBytes := min(softCapAvailableBytes, hardCapAvailableBytes)
		availableCount := min(softCapAvailableCount, hardCapAvailableCount)

		// Check if the request can be satisfied
		if additionalBytes <= availableBytes && additionalCount <= availableCount {
			// Success - calculate allocation breakdown
			freeBytesUsage := min(additionalBytes, freeSpaceBytes)
			fairShareBytesUsage := additionalBytes - freeBytesUsage
			freeCountUsage := min(additionalCount, freeSpaceCount)
			fairShareCountUsage := additionalCount - freeCountUsage

			return CapEnforcementResult{
				Error:          nil,
				FreeBytes:      freeBytesUsage,
				FairShareBytes: fairShareBytesUsage,
				FreeCount:      freeCountUsage,
				FairShareCount: fairShareCountUsage,
				AvailableBytes: availableBytes,
				AvailableCount: availableCount,
			}
		}

		// Determine which constraint failed
		var err error
		if additionalBytes > hardCapAvailableBytes || additionalCount > hardCapAvailableCount {
			// Hard cap constraint failed
			if additionalBytes > hardCapAvailableBytes {
				err = ErrBytesBudgetExceeded
			} else {
				err = ErrCountBudgetExceeded
			}
		} else {
			// Soft cap constraint failed
			if additionalBytes > softCapAvailableBytes {
				err = ErrBytesSoftCapExceeded
			} else {
				err = ErrCountSoftCapExceeded
			}
		}

		return CapEnforcementResult{
			Error:          err,
			FreeBytes:      0, // No allocation on failure
			FairShareBytes: 0, // No allocation on failure
			FreeCount:      0, // No allocation on failure
			FairShareCount: 0, // No allocation on failure
			AvailableBytes: availableBytes,
			AvailableCount: availableCount,
		}
	}

	// Soft caps disabled - only check hard cap
	availableBytes := hardCapAvailableBytes
	availableCount := hardCapAvailableCount

	if additionalBytes <= hardCapAvailableBytes && additionalCount <= hardCapAvailableCount {
		// Success - all allocation goes to fair share when soft caps disabled
		return CapEnforcementResult{
			Error:          nil,
			FreeBytes:      0, // No free space allocation when soft caps disabled
			FairShareBytes: additionalBytes,
			FreeCount:      0, // No free space allocation when soft caps disabled
			FairShareCount: additionalCount,
			AvailableBytes: availableBytes,
			AvailableCount: availableCount,
		}
	}

	// Hard cap exceeded
	var err error
	if additionalBytes > hardCapAvailableBytes {
		err = ErrBytesBudgetExceeded
	} else {
		err = ErrCountBudgetExceeded
	}

	return CapEnforcementResult{
		Error:          err,
		FreeBytes:      0, // No allocation on failure
		FairShareBytes: 0, // No allocation on failure
		FreeCount:      0, // No allocation on failure
		FairShareCount: 0, // No allocation on failure
		AvailableBytes: availableBytes,
		AvailableCount: availableCount,
	}
}
