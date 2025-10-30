package cache

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
)

func TestBudgetManager_ReserveForCache(t *testing.T) {
	tests := []struct {
		name              string
		capacityBytes     int
		capacityCount     int
		softCapThreshold  float64
		requestBytes      uint64
		requestCount      int64
		cacheID           string
		expectedError     error
		expectedUsedBytes uint64
		expectedUsedCount int64
		description       string
	}{
		{
			name:              "successful reserve with sufficient capacity",
			capacityBytes:     1000,
			capacityCount:     100,
			softCapThreshold:  1.0, // disabled
			requestBytes:      100,
			requestCount:      10,
			cacheID:           "cache1",
			expectedError:     nil,
			expectedUsedBytes: 100,
			expectedUsedCount: 10,
			description:       "Should successfully reserve when capacity is available",
		},
		{
			name:              "hard cap exceeded - bytes",
			capacityBytes:     100,
			capacityCount:     100,
			softCapThreshold:  1.0, // disabled
			requestBytes:      200,
			requestCount:      10,
			cacheID:           "cache1",
			expectedError:     ErrBytesBudgetExceeded,
			expectedUsedBytes: 0,
			expectedUsedCount: 0,
			description:       "Should fail when requesting more bytes than hard cap",
		},
		{
			name:              "hard cap exceeded - count",
			capacityBytes:     1000,
			capacityCount:     10,
			softCapThreshold:  1.0, // disabled
			requestBytes:      100,
			requestCount:      20,
			cacheID:           "cache1",
			expectedError:     ErrCountBudgetExceeded,
			expectedUsedBytes: 0,
			expectedUsedCount: 0,
			description:       "Should fail when requesting more count than hard cap",
		},
		{
			name:              "zero capacity - bytes",
			capacityBytes:     0,
			capacityCount:     100,
			softCapThreshold:  1.0,
			requestBytes:      10,
			requestCount:      1,
			cacheID:           "cache1",
			expectedError:     ErrBytesBudgetExceeded,
			expectedUsedBytes: 0,
			expectedUsedCount: 0,
			description:       "Should fail when bytes capacity is zero",
		},
		{
			name:              "zero capacity - count",
			capacityBytes:     1000,
			capacityCount:     0,
			softCapThreshold:  1.0,
			requestBytes:      10,
			requestCount:      1,
			cacheID:           "cache1",
			expectedError:     ErrCountBudgetExceeded,
			expectedUsedBytes: 0,
			expectedUsedCount: 0,
			description:       "Should fail when count capacity is zero",
		},
		{
			name:              "soft cap - free space allocation",
			capacityBytes:     1000,
			capacityCount:     100,
			softCapThreshold:  0.5, // 500 bytes free, 500 bytes fair share
			requestBytes:      100,
			requestCount:      10,
			cacheID:           "cache1",
			expectedError:     nil,
			expectedUsedBytes: 100,
			expectedUsedCount: 10,
			description:       "Should successfully allocate from free space when under threshold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewBudgetManager(
				"test",
				dynamicproperties.GetIntPropertyFn(tt.capacityBytes),
				dynamicproperties.GetIntPropertyFn(tt.capacityCount),
				AdmissionOptimistic,
				0,
				nil,
				testlogger.New(t),
				dynamicproperties.GetFloatPropertyFn(tt.softCapThreshold),
			)

			err := mgr.ReserveForCache(tt.cacheID, tt.requestBytes, tt.requestCount)

			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}

			assert.Equal(t, tt.expectedUsedBytes, mgr.UsedBytes(), "Used bytes mismatch")
			assert.Equal(t, tt.expectedUsedCount, mgr.UsedCount(), "Used count mismatch")
		})
	}
}

// Test soft cap with multiple caches - fair share enforcement
func TestBudgetManager_SoftCapFairShare(t *testing.T) {
	tests := []struct {
		name             string
		capacityBytes    int
		capacityCount    int
		softCapThreshold float64
		setup            func(mgr Manager)
		requestBytes     uint64
		requestCount     int64
		cacheID          string
		expectedError    error
		description      string
	}{
		{
			name:             "request within fair share - single active cache",
			capacityBytes:    1000,
			capacityCount:    100,
			softCapThreshold: 0.5, // 500 free, 500 fair share
			setup: func(mgr Manager) {
				// Fill up the free space (threshold)
				mgr.ReserveForCache("cache1", 500, 50)
				// Now cache1 tries to allocate more, should use fair share
			},
			requestBytes:  300, // Within fair share for cache1 (500/1 = 500 available)
			requestCount:  30,
			cacheID:       "cache1",
			expectedError: nil, // Should succeed because fair share allows it (cache1 is only active cache)
			description:   "Single active cache should get full fair share capacity",
		},
		{
			name:             "request within fair share - multiple caches fair share",
			capacityBytes:    1000,
			capacityCount:    100,
			softCapThreshold: 0.5,
			setup: func(mgr Manager) {
				// Fill up the free space
				mgr.ReserveForCache("cache1", 500, 50)
				// Activate cache2 with some fair share usage
				mgr.ReserveForCache("cache2", 100, 10)
				// Now 2 active caches, fair share = 500/2 = 250 per cache
			},
			requestBytes:  120, // Should be within fair share for cache2 (250 - 100 existing = 150 available)
			requestCount:  10,  // Should be within fair share for cache2 (25 - 10 existing = 15 available)
			cacheID:       "cache2",
			expectedError: nil,
			description:   "Should succeed because cache is within per-cache fair share with multiple active caches",
		},
		{
			name:             "soft cap exceeded - multiple caches fair share",
			capacityBytes:    1000,
			capacityCount:    100,
			softCapThreshold: 0.5,
			setup: func(mgr Manager) {
				// Fill up the free space
				mgr.ReserveForCache("cache1", 500, 50)
				// Activate cache2 with some fair share usage
				mgr.ReserveForCache("cache2", 100, 10)
				// Now 2 active caches, fair share = 500/2 = 250 per cache
			},
			requestBytes:  200, // cache2 already has 100, requesting 200 more = 300 total > 250 fair share
			requestCount:  20,
			cacheID:       "cache2",
			expectedError: ErrBytesSoftCapExceeded,
			description:   "Should fail when exceeding per-cache fair share with multiple active caches",
		},
		{
			name:             "soft cap exceeded - count fair share violation",
			capacityBytes:    1000,
			capacityCount:    100,
			softCapThreshold: 0.5,
			setup: func(mgr Manager) {
				// Fill up the free space
				mgr.ReserveForCache("cache1", 500, 50)
				// Activate cache2
				mgr.ReserveForCache("cache2", 100, 10)
				// Now 2 active caches, fair share count = 50/2 = 25 per cache
			},
			requestBytes:  50, // bytes within limit
			requestCount:  20, // cache2 already has 10, requesting 20 more = 30 total > 25 fair share
			cacheID:       "cache2",
			expectedError: ErrCountSoftCapExceeded,
			description:   "Should fail when exceeding per-cache count fair share",
		},
		{
			name:             "soft cap exceeded - bytes fair share violation",
			capacityBytes:    1000,
			capacityCount:    100,
			softCapThreshold: 0.5,
			setup: func(mgr Manager) {
				// Fill up the free space
				mgr.ReserveForCache("cache1", 500, 50)
				// Activate cache2
				mgr.ReserveForCache("cache2", 100, 10)
				// Now 2 active caches, fair share count = 50/2 = 25 per cache
			},
			requestBytes:  200, // cache2 already has 100, requesting 200 more = 300 total > 250 fair share
			requestCount:  1,   // count within limit
			cacheID:       "cache2",
			expectedError: ErrBytesSoftCapExceeded,
			description:   "Should fail when exceeding per-cache bytes fair share",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewBudgetManager(
				"test",
				dynamicproperties.GetIntPropertyFn(tt.capacityBytes),
				dynamicproperties.GetIntPropertyFn(tt.capacityCount),
				AdmissionOptimistic,
				0,
				nil,
				testlogger.New(t),
				dynamicproperties.GetFloatPropertyFn(tt.softCapThreshold),
			)

			if tt.setup != nil {
				tt.setup(mgr)
			}

			err := mgr.ReserveForCache(tt.cacheID, tt.requestBytes, tt.requestCount)

			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// Test releasing capacity from fair share tracking
func TestBudgetManager_ReleaseFairShareTracking(t *testing.T) {
	mgr := NewBudgetManager(
		"test",
		dynamicproperties.GetIntPropertyFn(1000), // 1000 bytes capacity
		dynamicproperties.GetIntPropertyFn(100),  // 100 count capacity
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(t),
		dynamicproperties.GetFloatPropertyFn(0.5), // 500 free, 500 fair share
	).(*manager)

	// Fill up the free space (500 bytes)
	err := mgr.ReserveForCache("cache1", 500, 50)
	assert.NoError(t, err, "Should reserve free space successfully")

	// Now allocate from fair share (cache1 uses fair share portion)
	err = mgr.ReserveForCache("cache1", 200, 20)
	assert.NoError(t, err, "Should reserve from fair share successfully")

	// Total usage: 700 bytes (500 free + 200 fair share)
	assert.Equal(t, uint64(700), mgr.UsedBytes(), "Total used should be 700")

	// Check fair share tracking for cache1
	cacheUsage := mgr.getCacheUsage("cache1")
	fairShareBytes := cacheUsage.fairShareCapacityBytes
	assert.Equal(t, uint64(200), fairShareBytes, "Fair share bytes should be 200")

	// Release some capacity (100 bytes) - should deduct from fair share first
	mgr.ReleaseForCache("cache1", 100, 10)

	// Total usage should be 600 (not going to zero)
	assert.Equal(t, uint64(600), mgr.UsedBytes(), "Total used should be 600 after release")
	assert.Equal(t, int64(60), mgr.UsedCount(), "Total count should be 60 after release")

	// Fair share tracking should be reduced to 100
	fairShareBytes = cacheUsage.fairShareCapacityBytes
	assert.Equal(t, uint64(100), fairShareBytes, "Fair share bytes should be reduced to 100")

	// Release more (150 bytes) - should deduct remaining 100 from fair share, then 50 from free
	mgr.ReleaseForCache("cache1", 150, 15)

	// Total usage should be 450
	assert.Equal(t, uint64(450), mgr.UsedBytes(), "Total used should be 450 after second release")
	assert.Equal(t, int64(45), mgr.UsedCount(), "Total count should be 45 after second release")

	// Fair share tracking should be 0
	fairShareBytes = cacheUsage.fairShareCapacityBytes
	assert.Equal(t, uint64(0), fairShareBytes, "Fair share bytes should be 0 after releasing all fair share")
}

// Test individual Reserve/Release methods for bytes and count
func TestBudgetManager_IndividualReserveRelease(t *testing.T) {
	tests := []struct {
		name          string
		capacityBytes int
		capacityCount int
		operations    func(mgr Manager) error
		expectedError error
		description   string
	}{
		{
			name:          "ReserveBytesForCache success",
			capacityBytes: 1000,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				return mgr.ReserveBytesForCache("cache1", 100)
			},
			expectedError: nil,
			description:   "Should successfully reserve bytes only",
		},
		{
			name:          "ReserveBytesForCache exceeds capacity",
			capacityBytes: 100,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				return mgr.ReserveBytesForCache("cache1", 200)
			},
			expectedError: ErrBytesBudgetExceeded,
			description:   "Should fail when bytes exceed capacity",
		},
		{
			name:          "ReserveCountForCache success",
			capacityBytes: 1000,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				return mgr.ReserveCountForCache("cache1", 10)
			},
			expectedError: nil,
			description:   "Should successfully reserve count only",
		},
		{
			name:          "ReserveCountForCache exceeds capacity",
			capacityBytes: 1000,
			capacityCount: 10,
			operations: func(mgr Manager) error {
				return mgr.ReserveCountForCache("cache1", 20)
			},
			expectedError: ErrCountBudgetExceeded,
			description:   "Should fail when count exceeds capacity",
		},
		{
			name:          "ReleaseBytesForCache",
			capacityBytes: 1000,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				mgr.ReserveBytesForCache("cache1", 100)
				mgr.ReleaseBytesForCache("cache1", 50)
				if mgr.UsedBytes() != 50 {
					return assert.AnError
				}
				return nil
			},
			expectedError: nil,
			description:   "Should release bytes correctly",
		},
		{
			name:          "ReleaseCountForCache",
			capacityBytes: 1000,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				mgr.ReserveCountForCache("cache1", 10)
				mgr.ReleaseCountForCache("cache1", 5)
				if mgr.UsedCount() != 5 {
					return assert.AnError
				}
				return nil
			},
			expectedError: nil,
			description:   "Should release count correctly",
		},
		{
			name:          "ReserveCountForCache with negative value",
			capacityBytes: 1000,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				return mgr.ReserveCountForCache("cache1", -10)
			},
			expectedError: ErrInvalidValue,
			description:   "Should reject negative count values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewBudgetManager(
				"test",
				dynamicproperties.GetIntPropertyFn(tt.capacityBytes),
				dynamicproperties.GetIntPropertyFn(tt.capacityCount),
				AdmissionOptimistic,
				0,
				nil,
				testlogger.New(t),
				nil,
			)

			err := tt.operations(mgr)

			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// Test Strict admission mode
func TestBudgetManager_StrictAdmission(t *testing.T) {
	tests := []struct {
		name          string
		capacityBytes int
		capacityCount int
		operations    func(mgr Manager) error
		expectedError error
		description   string
	}{
		{
			name:          "strict mode - reserve within capacity",
			capacityBytes: 1000,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				return mgr.ReserveForCache("cache1", 100, 10)
			},
			expectedError: nil,
			description:   "Should successfully reserve in strict mode when capacity available",
		},
		{
			name:          "strict mode - bytes exceed capacity",
			capacityBytes: 100,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				return mgr.ReserveForCache("cache1", 200, 10)
			},
			expectedError: ErrBytesBudgetExceeded,
			description:   "Should fail in strict mode when bytes exceed capacity",
		},
		{
			name:          "strict mode - count exceed capacity",
			capacityBytes: 1000,
			capacityCount: 10,
			operations: func(mgr Manager) error {
				return mgr.ReserveForCache("cache1", 100, 20)
			},
			expectedError: ErrCountBudgetExceeded,
			description:   "Should fail in strict mode when count exceeds capacity",
		},
		{
			name:          "strict mode - prevents temporary overshoot",
			capacityBytes: 100,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				// First reserve should succeed
				if err := mgr.ReserveForCache("cache1", 60, 60); err != nil {
					return err
				}
				// Second reserve should fail (would exceed if allowed)
				return mgr.ReserveForCache("cache2", 50, 50)
			},
			expectedError: ErrBytesBudgetExceeded,
			description:   "Strict mode should prevent any overshoot attempts",
		},
		{
			name:          "strict mode - negative count value",
			capacityBytes: 1000,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				return mgr.ReserveCountForCache("cache1", -10)
			},
			expectedError: ErrInvalidValue,
			description:   "Strict mode should reject negative count values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewBudgetManager(
				"test",
				dynamicproperties.GetIntPropertyFn(tt.capacityBytes),
				dynamicproperties.GetIntPropertyFn(tt.capacityCount),
				AdmissionStrict, // Use strict mode
				0,
				nil,
				testlogger.New(t),
				nil,
			)

			err := tt.operations(mgr)

			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// Test rollback behavior when partial reservations fail
func TestBudgetManager_Rollback(t *testing.T) {
	tests := []struct {
		name          string
		capacityBytes int
		capacityCount int
		admissionMode AdmissionMode
		operations    func(mgr Manager) (error, uint64, int64)
		expectedError error
		expectedBytes uint64
		expectedCount int64
		description   string
	}{
		{
			name:          "rollback bytes when count fails",
			capacityBytes: 1000,
			capacityCount: 10,
			admissionMode: AdmissionOptimistic,
			operations: func(mgr Manager) (error, uint64, int64) {
				err := mgr.ReserveForCache("cache1", 100, 20)
				return err, mgr.UsedBytes(), mgr.UsedCount()
			},
			expectedError: ErrCountBudgetExceeded,
			expectedBytes: 0,
			expectedCount: 0,
			description:   "Should rollback bytes when count reservation fails",
		},
		{
			name:          "no rollback needed when bytes fail first",
			capacityBytes: 100,
			capacityCount: 1000,
			admissionMode: AdmissionOptimistic,
			operations: func(mgr Manager) (error, uint64, int64) {
				err := mgr.ReserveForCache("cache1", 200, 10)
				return err, mgr.UsedBytes(), mgr.UsedCount()
			},
			expectedError: ErrBytesBudgetExceeded,
			expectedBytes: 0,
			expectedCount: 0,
			description:   "No rollback needed when bytes fail before count is reserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewBudgetManager(
				"test",
				dynamicproperties.GetIntPropertyFn(tt.capacityBytes),
				dynamicproperties.GetIntPropertyFn(tt.capacityCount),
				tt.admissionMode,
				0,
				nil,
				testlogger.New(t),
				nil,
			)

			err, usedBytes, usedCount := tt.operations(mgr)

			assert.Equal(t, tt.expectedError, err, tt.description)
			assert.Equal(t, tt.expectedBytes, usedBytes, "Used bytes should match expected after rollback")
			assert.Equal(t, tt.expectedCount, usedCount, "Used count should match expected")
		})
	}
}

func TestBudgetManager_BytesOverflow(t *testing.T) {
	tests := []struct {
		name          string
		capacityBytes uint64
		capacityCount int64
		admission     AdmissionMode
		setupUsage    uint64
		reserveAmount uint64
		expectedError error
		description   string
	}{
		{
			name:          "strict mode - overflow detection",
			capacityBytes: math.MaxUint64,
			capacityCount: 100,
			admission:     AdmissionStrict,
			setupUsage:    math.MaxUint64 - 100,
			reserveAmount: 150,
			expectedError: ErrOverflow,
			description:   "Strict mode should detect overflow when old + n > MaxUint64",
		},
		{
			name:          "optimistic mode - overflow detection via wraparound",
			capacityBytes: math.MaxUint64,
			capacityCount: 100,
			admission:     AdmissionOptimistic,
			setupUsage:    math.MaxUint64 - 50,
			reserveAmount: 75,
			expectedError: ErrOverflow,
			description:   "Optimistic mode should detect wraparound when newVal < n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewBudgetManager(
				"test",
				dynamicproperties.GetIntPropertyFn(int(tt.capacityBytes)),
				dynamicproperties.GetIntPropertyFn(int(tt.capacityCount)),
				tt.admission,
				0,
				nil,
				testlogger.New(t),
				nil,
			)

			m := mgr.(*manager)
			atomic.StoreUint64(&m.usedBytes, tt.setupUsage)

			err := m.reserveBytes(tt.reserveAmount)
			assert.Equal(t, tt.expectedError, err, tt.description)
		})
	}
}

func TestBudgetManager_ReserveOrReclaimSelfRelease(t *testing.T) {
	tests := []struct {
		name          string
		capacityBytes int
		capacityCount int
		retriable     bool
		requestBytes  uint64
		requestCount  int64
		setupUsage    func(mgr Manager)
		reclaimFunc   ReclaimSelfRelease
		expectedError error
		expectedBytes uint64
		expectedCount int64
		description   string
	}{
		{
			name:          "immediate success - no reclaim needed",
			capacityBytes: 1000,
			capacityCount: 100,
			retriable:     true,
			requestBytes:  100,
			requestCount:  10,
			setupUsage:    nil,
			reclaimFunc:   nil,
			expectedError: nil,
			expectedBytes: 100,
			expectedCount: 10,
			description:   "Should succeed immediately when capacity is available",
		},
		{
			name:          "non-retriable failure",
			capacityBytes: 1000,
			capacityCount: 100,
			retriable:     false,
			requestBytes:  500,
			requestCount:  50,
			setupUsage: func(mgr Manager) {
				mgr.ReserveForCache("other_cache", 600, 60)
			},
			reclaimFunc:   nil,
			expectedError: ErrBytesBudgetExceeded,
			expectedBytes: 600,
			expectedCount: 60,
			description:   "Should fail immediately when retriable is false",
		},
		{
			name:          "reclaim with self-release",
			capacityBytes: 1000,
			capacityCount: 100,
			retriable:     true,
			requestBytes:  500,
			requestCount:  50,
			setupUsage: func(mgr Manager) {
				mgr.ReserveForCache("cache1", 600, 60)
			},
			reclaimFunc: func(needBytes uint64, needCount int64) {
			},
			expectedError: nil,
			expectedBytes: 1000,
			expectedCount: 100,
			description:   "Should succeed after reclaim callback (cache releases its own items)",
		},
		{
			name:          "context cancellation",
			capacityBytes: 1000,
			capacityCount: 100,
			retriable:     true,
			requestBytes:  500,
			requestCount:  50,
			setupUsage: func(mgr Manager) {
				mgr.ReserveForCache("cache1", 1000, 100)
			},
			reclaimFunc: func(needBytes uint64, needCount int64) {
			},
			expectedError: context.Canceled,
			expectedBytes: 1000,
			expectedCount: 100,
			description:   "Should return context error when context is cancelled",
		},
		{
			name:          "insufficient cache budget for reclaim",
			capacityBytes: 1000,
			capacityCount: 100,
			retriable:     true,
			requestBytes:  900,
			requestCount:  90,
			setupUsage: func(mgr Manager) {
				mgr.ReserveForCache("cache1", 100, 10)
				mgr.ReserveForCache("other_cache", 200, 20)
			},
			reclaimFunc: func(needBytes uint64, needCount int64) {
			},
			expectedError: ErrInsufficientUsageToReclaim,
			expectedBytes: 300,
			expectedCount: 30,
			description:   "Should fail when cache doesn't have enough to reclaim",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewBudgetManager(
				"test",
				dynamicproperties.GetIntPropertyFn(tt.capacityBytes),
				dynamicproperties.GetIntPropertyFn(tt.capacityCount),
				AdmissionOptimistic,
				0,
				nil,
				testlogger.New(t),
				nil,
			)

			if tt.setupUsage != nil {
				tt.setupUsage(mgr)
			}

			var reclaimCalled bool
			var reclaimFunc ReclaimSelfRelease
			if tt.reclaimFunc != nil {
				reclaimFunc = func(needBytes uint64, needCount int64) {
					reclaimCalled = true
					mgr.ReleaseForCache("cache1", needBytes, needCount)
				}
			}

			ctx := context.Background()
			if tt.expectedError == context.Canceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := mgr.ReserveOrReclaimSelfRelease(ctx, "cache1", tt.requestBytes, tt.requestCount, tt.retriable, reclaimFunc)

			assert.Equal(t, tt.expectedError, err, tt.description)
			if tt.reclaimFunc != nil && tt.expectedError == nil {
				assert.True(t, reclaimCalled, "Reclaim function should be called")
			}

			assert.Equal(t, tt.expectedBytes, mgr.UsedBytes(), "Used bytes mismatch")
			assert.Equal(t, tt.expectedCount, mgr.UsedCount(), "Used count mismatch")
		})
	}
}

func TestBudgetManager_ReserveOrReclaimManagerRelease(t *testing.T) {
	tests := []struct {
		name          string
		capacityBytes int
		capacityCount int
		retriable     bool
		requestBytes  uint64
		requestCount  int64
		setupUsage    func(mgr Manager)
		reclaimFunc   ReclaimManagerRelease
		expectedError error
		expectedBytes uint64
		expectedCount int64
		description   string
	}{
		{
			name:          "immediate success - no reclaim needed",
			capacityBytes: 1000,
			capacityCount: 100,
			retriable:     true,
			requestBytes:  100,
			requestCount:  10,
			setupUsage:    nil,
			reclaimFunc:   nil,
			expectedError: nil,
			expectedBytes: 100,
			expectedCount: 10,
			description:   "Should succeed immediately when capacity is available",
		},
		{
			name:          "non-retriable failure",
			capacityBytes: 1000,
			capacityCount: 100,
			retriable:     false,
			requestBytes:  500,
			requestCount:  50,
			setupUsage: func(mgr Manager) {
				mgr.ReserveForCache("other_cache", 600, 60)
			},
			reclaimFunc:   nil,
			expectedError: ErrBytesBudgetExceeded,
			expectedBytes: 600,
			expectedCount: 60,
			description:   "Should fail immediately when retriable is false",
		},
		{
			name:          "reclaim with manager release",
			capacityBytes: 1000,
			capacityCount: 100,
			retriable:     true,
			requestBytes:  500,
			requestCount:  50,
			setupUsage: func(mgr Manager) {
				mgr.ReserveForCache("cache1", 600, 60)
			},
			reclaimFunc: func(needBytes uint64, needCount int64) (uint64, int64) {
				return needBytes, needCount
			},
			expectedError: nil,
			expectedBytes: 1000,
			expectedCount: 100,
			description:   "Should succeed after reclaim callback (manager releases items)",
		},
		{
			name:          "context cancellation",
			capacityBytes: 1000,
			capacityCount: 100,
			retriable:     true,
			requestBytes:  500,
			requestCount:  50,
			setupUsage: func(mgr Manager) {
				mgr.ReserveForCache("cache1", 1000, 100)
			},
			reclaimFunc: func(needBytes uint64, needCount int64) (uint64, int64) {
				return needBytes, needCount
			},
			expectedError: context.Canceled,
			expectedBytes: 1000,
			expectedCount: 100,
			description:   "Should return context error when context is cancelled",
		},
		{
			name:          "insufficient cache budget for reclaim",
			capacityBytes: 1000,
			capacityCount: 100,
			retriable:     true,
			requestBytes:  900,
			requestCount:  90,
			setupUsage: func(mgr Manager) {
				mgr.ReserveForCache("cache1", 100, 10)
				mgr.ReserveForCache("other_cache", 200, 20)
			},
			reclaimFunc: func(needBytes uint64, needCount int64) (uint64, int64) {
				return needBytes, needCount
			},
			expectedError: ErrInsufficientUsageToReclaim,
			expectedBytes: 300,
			expectedCount: 30,
			description:   "Should fail when cache doesn't have enough to reclaim",
		},
		{
			name:          "reclaim returns zero",
			capacityBytes: 1000,
			capacityCount: 100,
			retriable:     true,
			requestBytes:  500,
			requestCount:  50,
			setupUsage: func(mgr Manager) {
				mgr.ReserveForCache("cache1", 600, 60)
			},
			reclaimFunc: func(needBytes uint64, needCount int64) (uint64, int64) {
				return 0, 0
			},
			expectedError: ErrBytesBudgetExceeded,
			expectedBytes: 600,
			expectedCount: 60,
			description:   "Should keep retrying when reclaim returns zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewBudgetManager(
				"test",
				dynamicproperties.GetIntPropertyFn(tt.capacityBytes),
				dynamicproperties.GetIntPropertyFn(tt.capacityCount),
				AdmissionOptimistic,
				0,
				nil,
				testlogger.New(t),
				nil,
			)

			if tt.setupUsage != nil {
				tt.setupUsage(mgr)
			}

			var reclaimCalled bool
			var reclaimFunc ReclaimManagerRelease
			if tt.reclaimFunc != nil {
				reclaimFunc = func(needBytes uint64, needCount int64) (uint64, int64) {
					reclaimCalled = true
					return tt.reclaimFunc(needBytes, needCount)
				}
			}

			ctx := context.Background()
			if tt.expectedError == context.Canceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			} else if tt.name == "reclaim returns zero" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 10*time.Millisecond)
				defer cancel()
			}

			err := mgr.ReserveOrReclaimManagerRelease(ctx, "cache1", tt.requestBytes, tt.requestCount, tt.retriable, reclaimFunc)

			if tt.name == "reclaim returns zero" {
				assert.Error(t, err, tt.description)
			} else {
				assert.Equal(t, tt.expectedError, err, tt.description)
			}
			if tt.reclaimFunc != nil && tt.expectedError == nil {
				assert.True(t, reclaimCalled, "Reclaim function should be called")
			}

			assert.Equal(t, tt.expectedBytes, mgr.UsedBytes(), "Used bytes mismatch")
			assert.Equal(t, tt.expectedCount, mgr.UsedCount(), "Used count mismatch")
		})
	}
}

func TestBudgetManager_CapacityCount(t *testing.T) {
	tests := []struct {
		name          string
		maxCount      int
		expectedCount int64
		description   string
	}{
		{
			name:          "positive capacity",
			maxCount:      100,
			expectedCount: 100,
			description:   "Should return the configured count capacity",
		},
		{
			name:          "zero capacity",
			maxCount:      0,
			expectedCount: 0,
			description:   "Should return 0 when capacity is 0",
		},
		{
			name:          "negative capacity (unlimited)",
			maxCount:      -1,
			expectedCount: math.MaxInt64,
			description:   "Should return MaxInt64 when capacity is negative (unlimited)",
		},
		{
			name:          "nil maxCount defaults to unlimited",
			maxCount:      -2,
			expectedCount: math.MaxInt64,
			description:   "Should return MaxInt64 when maxCount is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var maxCountFn dynamicproperties.IntPropertyFn
			if tt.maxCount == -2 {
				maxCountFn = nil
			} else {
				maxCountFn = dynamicproperties.GetIntPropertyFn(tt.maxCount)
			}

			mgr := NewBudgetManager(
				"test",
				dynamicproperties.GetIntPropertyFn(1000),
				maxCountFn,
				AdmissionOptimistic,
				0,
				nil,
				testlogger.New(t),
				nil,
			)

			assert.Equal(t, tt.expectedCount, mgr.CapacityCount(), tt.description)
		})
	}
}

func TestBudgetManager_AvailableBytes(t *testing.T) {
	tests := []struct {
		name          string
		capacityBytes int
		usedBytes     uint64
		expectedAvail uint64
		description   string
	}{
		{
			name:          "full capacity available",
			capacityBytes: 1000,
			usedBytes:     0,
			expectedAvail: 1000,
			description:   "Should return full capacity when nothing is used",
		},
		{
			name:          "partial capacity available",
			capacityBytes: 1000,
			usedBytes:     300,
			expectedAvail: 700,
			description:   "Should return remaining capacity",
		},
		{
			name:          "no capacity available - fully used",
			capacityBytes: 1000,
			usedBytes:     1000,
			expectedAvail: 0,
			description:   "Should return 0 when fully used",
		},
		{
			name:          "no capacity available - over capacity",
			capacityBytes: 1000,
			usedBytes:     1200,
			expectedAvail: 0,
			description:   "Should return 0 when used exceeds capacity",
		},
		{
			name:          "zero capacity",
			capacityBytes: 0,
			usedBytes:     0,
			expectedAvail: 0,
			description:   "Should return 0 when capacity is 0",
		},
		{
			name:          "nil maxBytes defaults to unlimited",
			capacityBytes: -2,
			usedBytes:     1000,
			expectedAvail: math.MaxUint64 - 1000,
			description:   "Should return MaxUint64 - used when maxBytes is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var maxBytesFn dynamicproperties.IntPropertyFn
			if tt.capacityBytes == -2 {
				maxBytesFn = nil
			} else {
				maxBytesFn = dynamicproperties.GetIntPropertyFn(tt.capacityBytes)
			}

			mgr := NewBudgetManager(
				"test",
				maxBytesFn,
				dynamicproperties.GetIntPropertyFn(100),
				AdmissionOptimistic,
				0,
				nil,
				testlogger.New(t),
				nil,
			)

			if tt.usedBytes > 0 {
				atomic.StoreUint64(&mgr.(*manager).usedBytes, tt.usedBytes)
			}

			assert.Equal(t, tt.expectedAvail, mgr.(*manager).AvailableBytes(), tt.description)
		})
	}
}

func TestBudgetManager_AvailableCount(t *testing.T) {
	tests := []struct {
		name          string
		capacityCount int
		usedCount     int64
		expectedAvail int64
		description   string
	}{
		{
			name:          "full capacity available",
			capacityCount: 100,
			usedCount:     0,
			expectedAvail: 100,
			description:   "Should return full capacity when nothing is used",
		},
		{
			name:          "partial capacity available",
			capacityCount: 100,
			usedCount:     30,
			expectedAvail: 70,
			description:   "Should return remaining capacity",
		},
		{
			name:          "no capacity available - fully used",
			capacityCount: 100,
			usedCount:     100,
			expectedAvail: 0,
			description:   "Should return 0 when fully used",
		},
		{
			name:          "no capacity available - over capacity",
			capacityCount: 100,
			usedCount:     120,
			expectedAvail: 0,
			description:   "Should return 0 when used exceeds capacity",
		},
		{
			name:          "zero capacity",
			capacityCount: 0,
			usedCount:     0,
			expectedAvail: 0,
			description:   "Should return 0 when capacity is 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewBudgetManager(
				"test",
				dynamicproperties.GetIntPropertyFn(1000),
				dynamicproperties.GetIntPropertyFn(tt.capacityCount),
				AdmissionOptimistic,
				0,
				nil,
				testlogger.New(t),
				nil,
			)

			if tt.usedCount > 0 {
				atomic.StoreInt64(&mgr.(*manager).usedCount, tt.usedCount)
			}

			assert.Equal(t, tt.expectedAvail, mgr.(*manager).AvailableCount(), tt.description)
		})
	}
}

func TestBudgetManager_InternalReserveMethods(t *testing.T) {
	tests := []struct {
		name          string
		capacityBytes int
		capacityCount int
		admissionMode AdmissionMode
		operation     func(*manager) error
		expectedError error
		description   string
	}{
		{
			name:          "reserveBytesStrict - capacity exceeded",
			capacityBytes: 100,
			capacityCount: 100,
			admissionMode: AdmissionStrict,
			operation: func(m *manager) error {
				return m.reserveBytesStrict(200)
			},
			expectedError: ErrBytesBudgetExceeded,
			description:   "Should fail when bytes exceed capacity in strict mode",
		},
		{
			name:          "reserveBytesStrict - success",
			capacityBytes: 100,
			capacityCount: 100,
			admissionMode: AdmissionStrict,
			operation: func(m *manager) error {
				return m.reserveBytesStrict(50)
			},
			expectedError: nil,
			description:   "Should succeed when bytes are within capacity in strict mode",
		},
		{
			name:          "reserveBytesStrict - overflow detection",
			capacityBytes: 1000,
			capacityCount: 100,
			admissionMode: AdmissionStrict,
			operation: func(m *manager) error {
				m.usedBytes = math.MaxUint64 - 100
				return m.reserveBytesStrict(150)
			},
			expectedError: ErrOverflow,
			description:   "Should detect overflow in strict mode",
		},
		{
			name:          "reserveCountStrict - capacity exceeded",
			capacityBytes: 100,
			capacityCount: 10,
			admissionMode: AdmissionStrict,
			operation: func(m *manager) error {
				return m.reserveCountStrict(20)
			},
			expectedError: ErrCountBudgetExceeded,
			description:   "Should fail when count exceeds capacity in strict mode",
		},
		{
			name:          "reserveCountStrict - success",
			capacityBytes: 100,
			capacityCount: 10,
			admissionMode: AdmissionStrict,
			operation: func(m *manager) error {
				return m.reserveCountStrict(5)
			},
			expectedError: nil,
			description:   "Should succeed when count is within capacity in strict mode",
		},
		{
			name:          "reserveCountStrict - overflow detection",
			capacityBytes: 1000,
			capacityCount: 1000,
			admissionMode: AdmissionStrict,
			operation: func(m *manager) error {
				m.usedCount = math.MaxInt64 - 10
				return m.reserveCountStrict(20)
			},
			expectedError: ErrOverflow,
			description:   "Should detect overflow in strict mode",
		},
		{
			name:          "reserveBytesOptimistic - capacity exceeded",
			capacityBytes: 100,
			capacityCount: 100,
			admissionMode: AdmissionOptimistic,
			operation: func(m *manager) error {
				return m.reserveBytesOptimistic(200)
			},
			expectedError: ErrBytesBudgetExceeded,
			description:   "Should fail when bytes exceed capacity in optimistic mode",
		},
		{
			name:          "reserveBytesOptimistic - success",
			capacityBytes: 100,
			capacityCount: 100,
			admissionMode: AdmissionOptimistic,
			operation: func(m *manager) error {
				return m.reserveBytesOptimistic(50)
			},
			expectedError: nil,
			description:   "Should succeed when bytes are within capacity in optimistic mode",
		},
		{
			name:          "reserveCountOptimistic - capacity exceeded",
			capacityBytes: 100,
			capacityCount: 10,
			admissionMode: AdmissionOptimistic,
			operation: func(m *manager) error {
				return m.reserveCountOptimistic(20)
			},
			expectedError: ErrCountBudgetExceeded,
			description:   "Should fail when count exceeds capacity in optimistic mode",
		},
		{
			name:          "reserveCountOptimistic - success",
			capacityBytes: 100,
			capacityCount: 10,
			admissionMode: AdmissionOptimistic,
			operation: func(m *manager) error {
				return m.reserveCountOptimistic(5)
			},
			expectedError: nil,
			description:   "Should succeed when count is within capacity in optimistic mode",
		},
		{
			name:          "reserveCountOptimistic - overflow wraparound",
			capacityBytes: 1000,
			capacityCount: 1000,
			admissionMode: AdmissionOptimistic,
			operation: func(m *manager) error {
				m.usedCount = math.MaxInt64 - 10
				return m.reserveCountOptimistic(20)
			},
			expectedError: ErrOverflow,
			description:   "Should detect wraparound overflow in optimistic mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewBudgetManager(
				"test",
				dynamicproperties.GetIntPropertyFn(tt.capacityBytes),
				dynamicproperties.GetIntPropertyFn(tt.capacityCount),
				tt.admissionMode,
				0,
				nil,
				testlogger.New(t),
				nil,
			)

			m := mgr.(*manager)
			err := tt.operation(m)

			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

func TestBudgetManager_CallbackMethods(t *testing.T) {
	tests := []struct {
		name          string
		capacityBytes int
		capacityCount int
		operation     func(mgr Manager) error
		expectedError error
		expectedBytes uint64
		expectedCount int64
		description   string
	}{
		{
			name:          "ReserveWithCallback - success",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				called := false
				err := mgr.ReserveWithCallback("cache1", 100, 10, func() error {
					called = true
					return nil
				})
				if !called {
					return errors.New("callback not called")
				}
				return err
			},
			expectedError: nil,
			expectedBytes: 100,
			expectedCount: 10,
			description:   "Should reserve and call callback on success",
		},
		{
			name:          "ReserveWithCallback - callback error releases",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				return mgr.ReserveWithCallback("cache1", 100, 10, func() error {
					return errors.New("callback failed")
				})
			},
			expectedError: errors.New("callback failed"),
			expectedBytes: 0,
			expectedCount: 0,
			description:   "Should release on callback error",
		},
		{
			name:          "ReserveWithCallback - reserve failure",
			capacityBytes: 100,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				return mgr.ReserveWithCallback("cache1", 200, 10, func() error {
					return errors.New("should not be called")
				})
			},
			expectedError: ErrBytesBudgetExceeded,
			expectedBytes: 0,
			expectedCount: 0,
			description:   "Should not call callback when reserve fails",
		},
		{
			name:          "ReserveBytesWithCallback - success",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				called := false
				err := mgr.ReserveBytesWithCallback("cache1", 100, func() error {
					called = true
					return nil
				})
				if !called {
					return errors.New("callback not called")
				}
				return err
			},
			expectedError: nil,
			expectedBytes: 100,
			expectedCount: 0,
			description:   "Should reserve bytes and call callback on success",
		},
		{
			name:          "ReserveBytesWithCallback - callback error releases",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				return mgr.ReserveBytesWithCallback("cache1", 100, func() error {
					return errors.New("callback failed")
				})
			},
			expectedError: errors.New("callback failed"),
			expectedBytes: 0,
			expectedCount: 0,
			description:   "Should release bytes on callback error",
		},
		{
			name:          "ReserveCountWithCallback - success",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				called := false
				err := mgr.ReserveCountWithCallback("cache1", 10, func() error {
					called = true
					return nil
				})
				if !called {
					return errors.New("callback not called")
				}
				return err
			},
			expectedError: nil,
			expectedBytes: 0,
			expectedCount: 10,
			description:   "Should reserve count and call callback on success",
		},
		{
			name:          "ReserveCountWithCallback - callback error releases",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				return mgr.ReserveCountWithCallback("cache1", 10, func() error {
					return errors.New("callback failed")
				})
			},
			expectedError: errors.New("callback failed"),
			expectedBytes: 0,
			expectedCount: 0,
			description:   "Should release count on callback error",
		},
		{
			name:          "ReleaseWithCallback - success",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				mgr.ReserveForCache("cache1", 100, 10)
				called := false
				err := mgr.ReleaseWithCallback("cache1", 100, 10, func() error {
					called = true
					return nil
				})
				if !called {
					return errors.New("callback not called")
				}
				return err
			},
			expectedError: nil,
			expectedBytes: 0,
			expectedCount: 0,
			description:   "Should call callback and release on success",
		},
		{
			name:          "ReleaseWithCallback - callback error does not release",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				mgr.ReserveForCache("cache1", 100, 10)
				return mgr.ReleaseWithCallback("cache1", 100, 10, func() error {
					return errors.New("callback failed")
				})
			},
			expectedError: errors.New("callback failed"),
			expectedBytes: 100,
			expectedCount: 10,
			description:   "Should not release when callback fails",
		},
		{
			name:          "ReleaseBytesWithCallback - success",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				mgr.ReserveBytesForCache("cache1", 100)
				called := false
				err := mgr.ReleaseBytesWithCallback("cache1", 100, func() error {
					called = true
					return nil
				})
				if !called {
					return errors.New("callback not called")
				}
				return err
			},
			expectedError: nil,
			expectedBytes: 0,
			expectedCount: 0,
			description:   "Should call callback and release bytes on success",
		},
		{
			name:          "ReleaseBytesWithCallback - callback error does not release",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				mgr.ReserveBytesForCache("cache1", 100)
				return mgr.ReleaseBytesWithCallback("cache1", 100, func() error {
					return errors.New("callback failed")
				})
			},
			expectedError: errors.New("callback failed"),
			expectedBytes: 100,
			expectedCount: 0,
			description:   "Should not release bytes when callback fails",
		},
		{
			name:          "ReleaseCountWithCallback - success",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				mgr.ReserveCountForCache("cache1", 10)
				called := false
				err := mgr.ReleaseCountWithCallback("cache1", 10, func() error {
					called = true
					return nil
				})
				if !called {
					return errors.New("callback not called")
				}
				return err
			},
			expectedError: nil,
			expectedBytes: 0,
			expectedCount: 0,
			description:   "Should call callback and release count on success",
		},
		{
			name:          "ReleaseCountWithCallback - callback error does not release",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				mgr.ReserveCountForCache("cache1", 10)
				return mgr.ReleaseCountWithCallback("cache1", 10, func() error {
					return errors.New("callback failed")
				})
			},
			expectedError: errors.New("callback failed"),
			expectedBytes: 0,
			expectedCount: 10,
			description:   "Should not release count when callback fails",
		},
		{
			name:          "ReserveOrReclaimSelfReleaseWithCallback - success",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				mgr.ReserveForCache("cache1", 600, 60)
				called := false
				callbackCalled := false
				err := mgr.ReserveOrReclaimSelfReleaseWithCallback(
					context.Background(),
					"cache1",
					500,
					50,
					true,
					func(needBytes uint64, needCount int64) {
						called = true
						mgr.ReleaseForCache("cache1", needBytes, needCount)
					},
					func() error {
						callbackCalled = true
						return nil
					},
				)
				if !called {
					return errors.New("reclaim not called")
				}
				if !callbackCalled {
					return errors.New("callback not called")
				}
				return err
			},
			expectedError: nil,
			expectedBytes: 1000,
			expectedCount: 100,
			description:   "Should reclaim, reserve and call callback on success",
		},
		{
			name:          "ReserveOrReclaimSelfReleaseWithCallback - callback error releases",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				mgr.ReserveForCache("cache1", 600, 60)
				return mgr.ReserveOrReclaimSelfReleaseWithCallback(
					context.Background(),
					"cache1",
					500,
					50,
					true,
					func(needBytes uint64, needCount int64) {
						mgr.ReleaseForCache("cache1", needBytes, needCount)
					},
					func() error {
						return errors.New("callback failed")
					},
				)
			},
			expectedError: errors.New("callback failed"),
			expectedBytes: 500,
			expectedCount: 50,
			description:   "Should release on callback error",
		},
		{
			name:          "ReserveOrReclaimManagerReleaseWithCallback - success",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				mgr.ReserveForCache("cache1", 600, 60)
				called := false
				callbackCalled := false
				err := mgr.ReserveOrReclaimManagerReleaseWithCallback(
					context.Background(),
					"cache1",
					500,
					50,
					true,
					func(needBytes uint64, needCount int64) (uint64, int64) {
						called = true
						return needBytes, needCount
					},
					func() error {
						callbackCalled = true
						return nil
					},
				)
				if !called {
					return errors.New("reclaim not called")
				}
				if !callbackCalled {
					return errors.New("callback not called")
				}
				return err
			},
			expectedError: nil,
			expectedBytes: 1000,
			expectedCount: 100,
			description:   "Should reclaim, reserve and call callback on success",
		},
		{
			name:          "ReserveOrReclaimManagerReleaseWithCallback - callback error releases",
			capacityBytes: 1000,
			capacityCount: 100,
			operation: func(mgr Manager) error {
				mgr.ReserveForCache("cache1", 600, 60)
				return mgr.ReserveOrReclaimManagerReleaseWithCallback(
					context.Background(),
					"cache1",
					500,
					50,
					true,
					func(needBytes uint64, needCount int64) (uint64, int64) {
						return needBytes, needCount
					},
					func() error {
						return errors.New("callback failed")
					},
				)
			},
			expectedError: errors.New("callback failed"),
			expectedBytes: 500,
			expectedCount: 50,
			description:   "Should release on callback error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewBudgetManager(
				"test",
				dynamicproperties.GetIntPropertyFn(tt.capacityBytes),
				dynamicproperties.GetIntPropertyFn(tt.capacityCount),
				AdmissionOptimistic,
				0,
				nil,
				testlogger.New(t),
				nil,
			)

			err := tt.operation(mgr)

			if tt.expectedError != nil {
				assert.Error(t, err, tt.description)
				assert.Equal(t, tt.expectedError.Error(), err.Error(), tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}

			assert.Equal(t, tt.expectedBytes, mgr.UsedBytes(), "Used bytes mismatch")
			assert.Equal(t, tt.expectedCount, mgr.UsedCount(), "Used count mismatch")
		})
	}
}
