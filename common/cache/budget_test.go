package cache

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	metricsmocks "github.com/uber/cadence/common/metrics/mocks"
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

			err := mgr.ReserveWithCallback(tt.cacheID, tt.requestBytes, tt.requestCount, func() error {
				return nil
			})

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
				mgr.ReserveWithCallback("cache1", 500, 50, func() error { return nil })
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
				mgr.ReserveWithCallback("cache1", 500, 50, func() error { return nil })
				// Activate cache2 with some fair share usage
				mgr.ReserveWithCallback("cache2", 100, 10, func() error { return nil })
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
				mgr.ReserveWithCallback("cache1", 500, 50, func() error { return nil })
				// Activate cache2 with some fair share usage
				mgr.ReserveWithCallback("cache2", 100, 10, func() error { return nil })
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
				mgr.ReserveWithCallback("cache1", 500, 50, func() error { return nil })
				// Activate cache2
				mgr.ReserveWithCallback("cache2", 100, 10, func() error { return nil })
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
				mgr.ReserveWithCallback("cache1", 500, 50, func() error { return nil })
				// Activate cache2
				mgr.ReserveWithCallback("cache2", 100, 10, func() error { return nil })
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

			err := mgr.ReserveWithCallback(tt.cacheID, tt.requestBytes, tt.requestCount, func() error { return nil })

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
	err := mgr.ReserveWithCallback("cache1", 500, 50, func() error { return nil })
	assert.NoError(t, err, "Should reserve free space successfully")

	// Now allocate from fair share (cache1 uses fair share portion)
	err = mgr.ReserveWithCallback("cache1", 200, 20, func() error { return nil })
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

	// Release more (150 bytes) - should deduct the remaining 100 from fair share, then 50 from free
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
				return mgr.ReserveBytesWithCallback("cache1", 100, func() error { return nil })
			},
			expectedError: nil,
			description:   "Should successfully reserve bytes only",
		},
		{
			name:          "ReserveBytesForCache exceeds capacity",
			capacityBytes: 100,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				return mgr.ReserveBytesWithCallback("cache1", 200, func() error { return nil })
			},
			expectedError: ErrBytesBudgetExceeded,
			description:   "Should fail when bytes exceed capacity",
		},
		{
			name:          "ReserveCountForCache success",
			capacityBytes: 1000,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				return mgr.ReserveCountWithCallback("cache1", 10, func() error { return nil })
			},
			expectedError: nil,
			description:   "Should successfully reserve count only",
		},
		{
			name:          "ReserveCountForCache exceeds capacity",
			capacityBytes: 1000,
			capacityCount: 10,
			operations: func(mgr Manager) error {
				return mgr.ReserveCountWithCallback("cache1", 20, func() error { return nil })
			},
			expectedError: ErrCountBudgetExceeded,
			description:   "Should fail when count exceeds capacity",
		},
		{
			name:          "ReleaseBytesForCache",
			capacityBytes: 1000,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				mgr.ReserveBytesWithCallback("cache1", 100, func() error { return nil })
				mgr.ReleaseBytesWithCallback("cache1", func() (uint64, error) { return 50, nil })
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
				mgr.ReserveCountWithCallback("cache1", 10, func() error { return nil })
				mgr.ReleaseCountWithCallback("cache1", func() (int64, error) { return 5, nil })
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
				return mgr.ReserveCountWithCallback("cache1", -10, func() error { return nil })
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
				return mgr.ReserveWithCallback("cache1", 100, 10, func() error { return nil })
			},
			expectedError: nil,
			description:   "Should successfully reserve in strict mode when capacity available",
		},
		{
			name:          "strict mode - bytes exceed capacity",
			capacityBytes: 100,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				return mgr.ReserveWithCallback("cache1", 200, 10, func() error { return nil })
			},
			expectedError: ErrBytesBudgetExceeded,
			description:   "Should fail in strict mode when bytes exceed capacity",
		},
		{
			name:          "strict mode - count exceed capacity",
			capacityBytes: 1000,
			capacityCount: 10,
			operations: func(mgr Manager) error {
				return mgr.ReserveWithCallback("cache1", 100, 20, func() error { return nil })
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
				if err := mgr.ReserveWithCallback("cache1", 60, 60, func() error { return nil }); err != nil {
					return err
				}
				// Second reserve should fail (would exceed if allowed)
				return mgr.ReserveWithCallback("cache2", 50, 50, func() error { return nil })
			},
			expectedError: ErrBytesBudgetExceeded,
			description:   "Strict mode should prevent any overshoot attempts",
		},
		{
			name:          "strict mode - negative count value",
			capacityBytes: 1000,
			capacityCount: 100,
			operations: func(mgr Manager) error {
				return mgr.ReserveCountWithCallback("cache1", -10, func() error { return nil })
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
				err := mgr.ReserveWithCallback("cache1", 100, 20, func() error { return nil })
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
				err := mgr.ReserveWithCallback("cache1", 200, 10, func() error { return nil })
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
				mgr.ReserveWithCallback("other_cache", 600, 60, func() error { return nil })
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
				mgr.ReserveWithCallback("cache1", 600, 60, func() error { return nil })
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
				mgr.ReserveWithCallback("cache1", 1000, 100, func() error { return nil })
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
				mgr.ReserveWithCallback("cache1", 100, 10, func() error { return nil })
				mgr.ReserveWithCallback("other_cache", 200, 20, func() error { return nil })
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
					mgr.ReleaseWithCallback("cache1", func() (uint64, int64, error) {
						return needBytes, needCount, nil
					})
				}
			}

			ctx := context.Background()
			if tt.expectedError == context.Canceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := mgr.(*manager).ReserveOrReclaimSelfReleaseWithCallback(ctx, "cache1", tt.requestBytes, tt.requestCount, tt.retriable, reclaimFunc, func() error { return nil })

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
				mgr.ReserveWithCallback("other_cache", 600, 60, func() error { return nil })
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
				mgr.ReserveWithCallback("cache1", 600, 60, func() error { return nil })
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
				mgr.ReserveWithCallback("cache1", 1000, 100, func() error { return nil })
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
				mgr.ReserveWithCallback("cache1", 100, 10, func() error { return nil })
				mgr.ReserveWithCallback("other_cache", 200, 20, func() error { return nil })
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
				mgr.ReserveWithCallback("cache1", 600, 60, func() error { return nil })
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

			err := mgr.(*manager).ReserveOrReclaimManagerReleaseWithCallback(ctx, "cache1", tt.requestBytes, tt.requestCount, tt.retriable, reclaimFunc, func() error { return nil })

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
				mgr.ReserveWithCallback("cache1", 100, 10, func() error { return nil })
				called := false
				err := mgr.ReleaseWithCallback("cache1", func() (uint64, int64, error) {
					called = true
					return 100, 10, nil
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
				mgr.ReserveWithCallback("cache1", 100, 10, func() error { return nil })
				return mgr.ReleaseWithCallback("cache1", func() (uint64, int64, error) {
					return 0, 0, errors.New("callback failed")
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
				mgr.ReserveBytesWithCallback("cache1", 100, func() error { return nil })
				called := false
				err := mgr.ReleaseBytesWithCallback("cache1", func() (uint64, error) {
					called = true
					return 100, nil
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
				mgr.ReserveBytesWithCallback("cache1", 100, func() error { return nil })
				return mgr.ReleaseBytesWithCallback("cache1", func() (uint64, error) {
					return 0, errors.New("callback failed")
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
				mgr.ReserveCountWithCallback("cache1", 10, func() error { return nil })
				called := false
				err := mgr.ReleaseCountWithCallback("cache1", func() (int64, error) {
					called = true
					return 10, nil
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
				mgr.ReserveCountWithCallback("cache1", 10, func() error { return nil })
				return mgr.ReleaseCountWithCallback("cache1", func() (int64, error) {
					return 0, errors.New("callback failed")
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
				mgr.ReserveWithCallback("cache1", 600, 60, func() error { return nil })
				called := false
				callbackCalled := false
				err := mgr.(*manager).ReserveOrReclaimSelfReleaseWithCallback(
					context.Background(),
					"cache1",
					500,
					50,
					true,
					func(needBytes uint64, needCount int64) {
						called = true
						mgr.ReleaseWithCallback("cache1", func() (uint64, int64, error) { return needBytes, needCount, nil })
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
				mgr.ReserveWithCallback("cache1", 600, 60, func() error { return nil })
				return mgr.(*manager).ReserveOrReclaimSelfReleaseWithCallback(
					context.Background(),
					"cache1",
					500,
					50,
					true,
					func(needBytes uint64, needCount int64) {
						mgr.ReleaseWithCallback("cache1", func() (uint64, int64, error) { return needBytes, needCount, nil })
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
				mgr.ReserveWithCallback("cache1", 600, 60, func() error { return nil })
				called := false
				callbackCalled := false
				err := mgr.(*manager).ReserveOrReclaimManagerReleaseWithCallback(
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
				mgr.ReserveWithCallback("cache1", 600, 60, func() error { return nil })
				return mgr.(*manager).ReserveOrReclaimManagerReleaseWithCallback(
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

func TestBudgetManager_ConcurrentCorrectness(t *testing.T) {
	mgr := NewBudgetManager(
		"test",
		dynamicproperties.GetIntPropertyFn(1000000),
		dynamicproperties.GetIntPropertyFn(100000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(t),
		nil,
	)

	const (
		numGoroutines = 100
		numOperations = 1000
		reserveBytes  = 100
		reserveCount  = 1
	)

	// Track total operations for verification
	var totalReservedBytes atomic.Uint64
	var totalReservedCount atomic.Int64
	var totalReleasedBytes atomic.Uint64
	var totalReleasedCount atomic.Int64

	// Run concurrent reserve/release operations
	// Each goroutine will reserve some and release some (but not all)
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			cacheID := fmt.Sprintf("cache%d", goroutineID%10)

			for i := 0; i < numOperations; i++ {
				// Reserve
				err := mgr.(*manager).ReserveForCache(cacheID, reserveBytes, reserveCount)
				if err == nil {
					totalReservedBytes.Add(reserveBytes)
					totalReservedCount.Add(reserveCount)

					// Release only half the time to maintain non-zero state
					if i%2 == 0 {
						mgr.(*manager).ReleaseForCache(cacheID, reserveBytes, reserveCount)
						totalReleasedBytes.Add(reserveBytes)
						totalReleasedCount.Add(reserveCount)
					}
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify invariants without assuming zero state
	// 1. Manager's used bytes should equal total reserved - total released
	expectedBytes := totalReservedBytes.Load() - totalReleasedBytes.Load()
	expectedCount := totalReservedCount.Load() - totalReleasedCount.Load()

	assert.Equal(t, expectedBytes, mgr.UsedBytes(),
		"Used bytes should equal total reserved minus total released")
	assert.Equal(t, expectedCount, mgr.UsedCount(),
		"Used count should equal total reserved minus total released")

	// 2. Available capacity should equal total - used
	assert.Equal(t, uint64(1000000)-expectedBytes, mgr.(*manager).AvailableBytes(),
		"Available bytes should equal capacity minus used")
	assert.Equal(t, int64(100000)-expectedCount, mgr.(*manager).AvailableCount(),
		"Available count should equal capacity minus used")

	// 3. Active cache count should be > 0 since we have unreleased capacity
	if expectedBytes > 0 || expectedCount > 0 {
		assert.Greater(t, mgr.(*manager).getActiveCacheCount(), int64(0),
			"Should have active caches when capacity is in use")
	}

	// 4. Log final state for debugging
	t.Logf("Final state: reserved=%d bytes, released=%d bytes, used=%d bytes, active_caches=%d",
		totalReservedBytes.Load(), totalReleasedBytes.Load(), mgr.UsedBytes(),
		mgr.(*manager).getActiveCacheCount())
}

func TestBudgetManager_ConcurrentSoftCapCorrectness(t *testing.T) {
	mgr := NewBudgetManager(
		"test",
		dynamicproperties.GetIntPropertyFn(10000), // Smaller capacity
		dynamicproperties.GetIntPropertyFn(1000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(t),
		dynamicproperties.GetFloatPropertyFn(0.5), // 50% soft cap = 5000 bytes free
	)

	const (
		numGoroutines = 50
		numOperations = 100
		reserveBytes  = 200
		reserveCount  = 2
	)

	var totalReservedBytes atomic.Uint64
	var totalReservedCount atomic.Int64
	var totalReleasedBytes atomic.Uint64
	var totalReleasedCount atomic.Int64
	var totalSuccessful atomic.Int64
	var totalFailed atomic.Int64

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Try to reserve concurrently - soft cap should cause some failures
	// Release only half to maintain non-zero state
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			cacheID := fmt.Sprintf("cache%d", goroutineID%10)

			for i := 0; i < numOperations; i++ {
				err := mgr.(*manager).ReserveForCache(cacheID, reserveBytes, reserveCount)
				if err == nil {
					totalSuccessful.Add(1)
					totalReservedBytes.Add(reserveBytes)
					totalReservedCount.Add(reserveCount)

					// Release only half the time to maintain non-zero state
					if i%2 == 0 {
						mgr.(*manager).ReleaseForCache(cacheID, reserveBytes, reserveCount)
						totalReleasedBytes.Add(reserveBytes)
						totalReleasedCount.Add(reserveCount)
					}
				} else {
					totalFailed.Add(1)
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify invariants without assuming zero state
	expectedBytes := totalReservedBytes.Load() - totalReleasedBytes.Load()
	expectedCount := totalReservedCount.Load() - totalReleasedCount.Load()

	assert.Equal(t, expectedBytes, mgr.UsedBytes(),
		"Used bytes should equal total reserved minus total released")
	assert.Equal(t, expectedCount, mgr.UsedCount(),
		"Used count should equal total reserved minus total released")

	// Available capacity should equal total - used
	assert.Equal(t, uint64(10000)-expectedBytes, mgr.(*manager).AvailableBytes(),
		"Available bytes should equal capacity minus used")
	assert.Equal(t, int64(1000)-expectedCount, mgr.(*manager).AvailableCount(),
		"Available count should equal capacity minus used")

	// Active cache count should be > 0 since we have unreleased capacity
	if expectedBytes > 0 || expectedCount > 0 {
		assert.Greater(t, mgr.(*manager).getActiveCacheCount(), int64(0),
			"Should have active caches when capacity is in use")
		assert.NotZero(t, mgr.UsedBytes(), "Should have non-zero bytes in use")
		assert.NotZero(t, mgr.UsedCount(), "Should have non-zero count in use")
	}

	// Log statistics
	t.Logf("Final state: reserved=%d bytes, released=%d bytes, used=%d bytes, active_caches=%d",
		totalReservedBytes.Load(), totalReleasedBytes.Load(), mgr.UsedBytes(),
		mgr.(*manager).getActiveCacheCount())
	t.Logf("Operations: successful=%d, failed=%d, total=%d",
		totalSuccessful.Load(), totalFailed.Load(),
		totalSuccessful.Load()+totalFailed.Load())

	// At least some operations should have succeeded
	assert.Greater(t, totalSuccessful.Load(), int64(0),
		"Some operations should succeed")
	// Soft cap should have caused some failures
	assert.Greater(t, totalFailed.Load(), int64(0),
		"Soft cap should cause some failures")
}

func TestBudgetManager_ConcurrentActiveCacheCount(t *testing.T) {
	mgr := NewBudgetManager(
		"test",
		dynamicproperties.GetIntPropertyFn(1000000),
		dynamicproperties.GetIntPropertyFn(100000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(t),
		dynamicproperties.GetFloatPropertyFn(0.5),
	)

	const numCaches = 20

	// Activate all caches
	for i := 0; i < numCaches; i++ {
		cacheID := fmt.Sprintf("cache%d", i)
		err := mgr.(*manager).ReserveForCache(cacheID, 100, 1)
		assert.NoError(t, err)
	}

	// Verify active cache count
	assert.Equal(t, int64(numCaches), mgr.(*manager).getActiveCacheCount(),
		"All caches should be active")

	// Deactivate all caches concurrently
	var wg sync.WaitGroup
	wg.Add(numCaches)
	for i := 0; i < numCaches; i++ {
		go func(cacheID string) {
			defer wg.Done()
			mgr.(*manager).ReleaseForCache(cacheID, 100, 1)
		}(fmt.Sprintf("cache%d", i))
	}
	wg.Wait()

	// Verify all caches are inactive
	assert.Equal(t, int64(0), mgr.(*manager).getActiveCacheCount(),
		"All caches should be inactive after release")
	assert.Equal(t, uint64(0), mgr.UsedBytes(),
		"All bytes should be released")
}

func BenchmarkBudgetManager_ReserveRelease(b *testing.B) {
	mgr := NewBudgetManager(
		"benchmark",
		dynamicproperties.GetIntPropertyFn(1000000),
		dynamicproperties.GetIntPropertyFn(100000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(b),
		nil,
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cacheID := "cache" + string(rune(i%10))
			mgr.(*manager).ReserveForCache(cacheID, 100, 1)
			mgr.(*manager).ReleaseForCache(cacheID, 100, 1)
			i++
		}
	})
}

func BenchmarkBudgetManager_ReserveBytes(b *testing.B) {
	mgr := NewBudgetManager(
		"benchmark",
		dynamicproperties.GetIntPropertyFn(1000000),
		dynamicproperties.GetIntPropertyFn(100000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(b),
		nil,
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cacheID := "cache" + string(rune(i%10))
			mgr.(*manager).ReserveBytesForCache(cacheID, 100)
			mgr.(*manager).ReleaseBytesForCache(cacheID, 100)
			i++
		}
	})
}

func BenchmarkBudgetManager_ReserveCount(b *testing.B) {
	mgr := NewBudgetManager(
		"benchmark",
		dynamicproperties.GetIntPropertyFn(1000000),
		dynamicproperties.GetIntPropertyFn(100000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(b),
		nil,
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cacheID := "cache" + string(rune(i%10))
			mgr.(*manager).ReserveCountForCache(cacheID, 1)
			mgr.(*manager).ReleaseCountForCache(cacheID, 1)
			i++
		}
	})
}

func BenchmarkBudgetManager_ReserveWithCallback(b *testing.B) {
	mgr := NewBudgetManager(
		"benchmark",
		dynamicproperties.GetIntPropertyFn(1000000),
		dynamicproperties.GetIntPropertyFn(100000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(b),
		nil,
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cacheID := "cache" + string(rune(i%10))
			mgr.ReserveWithCallback(cacheID, 100, 1, func() error {
				return nil
			})
			mgr.(*manager).ReleaseForCache(cacheID, 100, 1)
			i++
		}
	})
}

func BenchmarkBudgetManager_ReleaseWithCallback(b *testing.B) {
	mgr := NewBudgetManager(
		"benchmark",
		dynamicproperties.GetIntPropertyFn(1000000),
		dynamicproperties.GetIntPropertyFn(100000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(b),
		nil,
	)

	for i := 0; i < 1000; i++ {
		cacheID := "cache" + string(rune(i%10))
		mgr.(*manager).ReserveForCache(cacheID, 100, 1)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cacheID := "cache" + string(rune(i%10))
			mgr.ReleaseWithCallback(cacheID, func() (uint64, int64, error) {
				return 100, 1, nil
			})
			mgr.(*manager).ReserveForCache(cacheID, 100, 1)
			i++
		}
	})
}

func BenchmarkBudgetManager_ReserveOrReclaimSelfRelease(b *testing.B) {
	mgr := NewBudgetManager(
		"benchmark",
		dynamicproperties.GetIntPropertyFn(10000),
		dynamicproperties.GetIntPropertyFn(1000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(b),
		nil,
	)

	mgr.(*manager).ReserveForCache("cache1", 9000, 900)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.(*manager).ReserveOrReclaimSelfRelease(
			context.Background(),
			"cache1",
			2000,
			200,
			true,
			func(needBytes uint64, needCount int64) {
				mgr.(*manager).ReleaseForCache("cache1", needBytes, needCount)
			},
		)
		mgr.(*manager).ReleaseForCache("cache1", 2000, 200)
	}
}

func BenchmarkBudgetManager_ReserveOrReclaimManagerRelease(b *testing.B) {
	mgr := NewBudgetManager(
		"benchmark",
		dynamicproperties.GetIntPropertyFn(10000),
		dynamicproperties.GetIntPropertyFn(1000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(b),
		nil,
	)

	mgr.(*manager).ReserveForCache("cache1", 9000, 900)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.(*manager).ReserveOrReclaimManagerRelease(
			context.Background(),
			"cache1",
			2000,
			200,
			true,
			func(needBytes uint64, needCount int64) (uint64, int64) {
				return needBytes, needCount
			},
		)
		mgr.(*manager).ReleaseForCache("cache1", 2000, 200)
	}
}

func BenchmarkBudgetManager_MultipleCaches(b *testing.B) {
	mgr := NewBudgetManager(
		"benchmark",
		dynamicproperties.GetIntPropertyFn(1000000),
		dynamicproperties.GetIntPropertyFn(100000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(b),
		nil,
	)

	numCaches := 100
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cacheID := "cache" + string(rune(i%numCaches))
			mgr.(*manager).ReserveForCache(cacheID, 1000, 10)
			mgr.(*manager).ReleaseForCache(cacheID, 1000, 10)
			i++
		}
	})
}

func BenchmarkBudgetManager_StrictMode(b *testing.B) {
	mgr := NewBudgetManager(
		"benchmark",
		dynamicproperties.GetIntPropertyFn(1000000),
		dynamicproperties.GetIntPropertyFn(100000),
		AdmissionStrict,
		0,
		nil,
		testlogger.New(b),
		nil,
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cacheID := "cache" + string(rune(i%10))
			mgr.(*manager).ReserveForCache(cacheID, 100, 1)
			mgr.(*manager).ReleaseForCache(cacheID, 100, 1)
			i++
		}
	})
}

func BenchmarkBudgetManager_SoftCap(b *testing.B) {
	mgr := NewBudgetManager(
		"benchmark",
		dynamicproperties.GetIntPropertyFn(1000000),
		dynamicproperties.GetIntPropertyFn(100000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(b),
		dynamicproperties.GetFloatPropertyFn(0.5),
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cacheID := fmt.Sprintf("cache%d", i%10)
			mgr.(*manager).ReserveForCache(cacheID, 100, 1)
			mgr.(*manager).ReleaseForCache(cacheID, 100, 1)
			i++
		}
	})
}

func BenchmarkBudgetManager_HighContention(b *testing.B) {
	mgr := NewBudgetManager(
		"benchmark",
		dynamicproperties.GetIntPropertyFn(100000),
		dynamicproperties.GetIntPropertyFn(10000),
		AdmissionOptimistic,
		0,
		nil,
		testlogger.New(b),
		nil,
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mgr.(*manager).ReserveForCache("shared-cache", 100, 1)
			mgr.(*manager).ReleaseForCache("shared-cache", 100, 1)
		}
	})
}

func TestBudgetManager_DisabledManager_NoMetricsEmitted(t *testing.T) {
	// Create a mock scope that will track all metric calls
	mockScope := &metricsmocks.Scope{}
	// Mock the Tagged call which returns itself
	mockScope.On("Tagged", mock.Anything).Return(mockScope)

	// Create manager with BOTH capacities disabled (set to 0)
	mgr := NewBudgetManager(
		"test-disabled-manager",
		dynamicproperties.GetIntPropertyFn(0), // maxBytes = 0 (disabled)
		dynamicproperties.GetIntPropertyFn(0), // maxCount = 0 (disabled)
		AdmissionOptimistic,
		0,
		mockScope,
		testlogger.New(t),
		dynamicproperties.GetFloatPropertyFn(0.8),
	)
	defer mgr.Stop()

	// Give time for any initial metrics to be emitted
	time.Sleep(100 * time.Millisecond)

	// Try to reserve capacity (should fail but shouldn't emit metrics)
	err := mgr.(*manager).ReserveForCache("cache1", 100, 1)
	assert.Error(t, err)

	// Try to release capacity (should not emit metrics)
	mgr.(*manager).ReleaseForCache("cache1", 100, 1)

	// Wait a bit to ensure no async metrics are emitted
	time.Sleep(100 * time.Millisecond)

	// Verify NO metrics were emitted (no calls to UpdateGauge or IncCounter)
	mockScope.AssertNotCalled(t, "UpdateGauge")
	mockScope.AssertNotCalled(t, "IncCounter")
}
