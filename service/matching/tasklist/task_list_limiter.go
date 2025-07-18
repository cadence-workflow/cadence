package tasklist

import (
	"context"
	"math"
	"sync"
	"time"

	"go.uber.org/atomic"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/service/matching/config"
)

const precision = 0.001

type taskListLimiter struct {
	backing           clock.Ratelimiter
	timeSource        clock.TimeSource
	scope             metrics.Scope
	ttl               time.Duration
	minBurst          int
	lock              sync.Mutex
	value             atomic.Float64
	lastReceivedValue atomic.Time
	lastUpdate        atomic.Time
	countPartitions   func() int
	partitions        int
}

func newTaskListLimiter(timeSource clock.TimeSource, scope metrics.Scope, config *config.TaskListConfig, numPartitions func() int) *taskListLimiter {
	l := &taskListLimiter{
		timeSource:      timeSource,
		scope:           scope,
		ttl:             config.TaskDispatchRPSTTL,
		countPartitions: numPartitions,
		minBurst:        config.MinTaskThrottlingBurstSize(),
	}
	l.value.Store(config.TaskDispatchRPS)
	l.partitions = numPartitions()
	now := timeSource.Now()
	l.lastUpdate.Store(now)
	l.lastReceivedValue.Store(now)
	l.backing = clock.NewRatelimiter(l.getLimitAndBurst())
	return l
}

func (l *taskListLimiter) Allow() bool {
	return l.backing.Allow()
}

func (l *taskListLimiter) Wait(ctx context.Context) error {
	return l.backing.Wait(ctx)
}

func (l *taskListLimiter) Reserve() clock.Reservation {
	return l.backing.Reserve()
}

func (l *taskListLimiter) Limit() rate.Limit {
	return l.backing.Limit()
}

func (l *taskListLimiter) ReportLimit(rps float64) {
	now := l.timeSource.Now()
	// Optimistically reject it without locking if it's >= current and within the TTL
	if now.Sub(l.lastUpdate.Load()) < l.ttl {
		current := l.value.Load()
		if rps > current {
			return
		}
		// If it's roughly equal to the current value, track the timestamp
		// We'll maintain this value in the next update unless something lower
		// comes along
		if math.Abs(current-rps) < precision {
			l.lastReceivedValue.Store(now)
			return
		}
		// else if rps < current, we try to update
	}
	l.tryUpdate(rps)
}

func (l *taskListLimiter) tryUpdate(rps float64) {
	l.lock.Lock()
	defer l.lock.Unlock()
	now := l.timeSource.Now()
	current := l.value.Load()
	lastUpdated := l.lastUpdate.Load()
	ttlElapsed := now.Sub(lastUpdated) >= l.ttl
	changed := false

	// Take the lower value, or if the ttl expired and haven't received the current low value within the TTL, take the
	// new value
	if rps < current || (ttlElapsed && !l.lastReceivedValue.Load().After(now.Add(-l.ttl))) {
		l.lastUpdate.Store(now)
		l.value.Store(rps)
		l.lastReceivedValue.Store(now)
		l.partitions = l.countPartitions()
		changed = true
		l.scope.UpdateGauge(metrics.RateLimitPerTaskListGauge, rps)
	} else if ttlElapsed {
		// If the TTL elapsed, recalculate the partition count in case it changed
		l.lastUpdate.Store(now)
		newPartitions := l.countPartitions()
		if newPartitions != l.partitions {
			l.partitions = newPartitions
			changed = true
		}
		l.scope.UpdateGauge(metrics.RateLimitPerTaskListGauge, current)
	}

	if changed {
		l.backing.SetLimitAndBurst(l.getLimitAndBurst())
	}
}

func (l *taskListLimiter) getLimitAndBurst() (rate.Limit, int) {
	rps := l.value.Load()
	rps = rps / float64(l.partitions)
	burst := max(int(math.Ceil(rps)), l.minBurst)
	if rps == 0 {
		burst = 0
	}
	return rate.Limit(rps), burst
}
