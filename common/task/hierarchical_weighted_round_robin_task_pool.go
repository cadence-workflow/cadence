package task

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
)

var (
	errWeightedKeyWeightMustBeGreaterThanZero = errors.New("weight must be greater than 0")
)

type HierarchicalWeightedRoundRobinTaskPoolOptions[K comparable, T Task] struct {
	BufferSize           int
	TaskToWeightedKeysFn func(T) []WeightedKey[K]
}

type hierarchicalWeightedRoundRobinTaskPoolImpl[K comparable, T Task] struct {
	sync.Mutex
	status     int32
	root       *iwrrNode[K, T]
	bufferSize int
	ctx        context.Context
	cancel     context.CancelFunc
	options    *HierarchicalWeightedRoundRobinTaskPoolOptions[K, T]
	logger     log.Logger
	timeSource clock.TimeSource
	wg         sync.WaitGroup

	doCleanupFn func(now time.Time, ttl time.Duration)
}

// newHierarchicalWeightedRoundRobinTaskPool creates a new hierarchical WRR task pool
func newHierarchicalWeightedRoundRobinTaskPool[K comparable, T Task](
	logger log.Logger,
	metricsClient metrics.Client,
	timeSource clock.TimeSource,
	options *HierarchicalWeightedRoundRobinTaskPoolOptions[K, T],
) *hierarchicalWeightedRoundRobinTaskPoolImpl[K, T] {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &hierarchicalWeightedRoundRobinTaskPoolImpl[K, T]{
		status:     common.DaemonStatusInitialized,
		root:       newiwrrNode[K, T](options.BufferSize),
		bufferSize: options.BufferSize,
		ctx:        ctx,
		cancel:     cancel,
		options:    options,
		logger:     logger,
		timeSource: timeSource,
	}
	pool.doCleanupFn = pool.doCleanup

	return pool
}

func (p *hierarchicalWeightedRoundRobinTaskPoolImpl[K, T]) Start() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	p.wg.Add(1)
	go p.cleanupLoop()

	p.logger.Info("Hierarchical weighted round robin task pool started.")
}

func (p *hierarchicalWeightedRoundRobinTaskPoolImpl[K, T]) Stop() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	p.cancel()
	p.wg.Wait()

	p.logger.Info("Hierarchical weighted round robin task pool stopped.")
}

func (p *hierarchicalWeightedRoundRobinTaskPoolImpl[K, T]) cleanupLoop() {
	defer p.wg.Done()

	ticker := p.timeSource.NewTicker(time.Duration((defaultIdleChannelTTLInSeconds / 2)) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Chan():
			now := p.timeSource.Now()
			ttl := time.Duration(defaultIdleChannelTTLInSeconds) * time.Second
			p.doCleanupFn(now, ttl)
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *hierarchicalWeightedRoundRobinTaskPoolImpl[K, T]) doCleanup(now time.Time, ttl time.Duration) {
	p.root.cleanup(now, ttl)
}

func (p *hierarchicalWeightedRoundRobinTaskPoolImpl[K, T]) Enqueue(task T) error {
	weightedKeys := p.options.TaskToWeightedKeysFn(task)
	if err := verifyWeightedKeys(weightedKeys); err != nil {
		return err
	}
	var err error
	p.root.executeAtPath(weightedKeys, p.bufferSize, func(c *TTLChannel[T]) int64 {
		c.IncRef()
		defer c.DecRef()
		select {
		case c.Chan() <- task:
			c.UpdateLastWriteTime(p.timeSource.Now())
			return 1
		case <-p.ctx.Done():
			err = p.ctx.Err()
			return 0
		}
	})
	return err
}

func (p *hierarchicalWeightedRoundRobinTaskPoolImpl[K, T]) TryEnqueue(task T) (bool, error) {
	weightedKeys := p.options.TaskToWeightedKeysFn(task)
	if err := verifyWeightedKeys(weightedKeys); err != nil {
		return false, err
	}
	var err error
	delta := p.root.executeAtPath(weightedKeys, p.bufferSize, func(c *TTLChannel[T]) int64 {
		c.IncRef()
		defer c.DecRef()
		select {
		case c.Chan() <- task:
			c.UpdateLastWriteTime(p.timeSource.Now())
			return 1
		case <-p.ctx.Done():
			err = p.ctx.Err()
			return 0
		default:
			return 0
		}
	})
	return delta > 0, err
}

func (p *hierarchicalWeightedRoundRobinTaskPoolImpl[K, T]) TryDequeue() (T, bool) {
	var zero T
	item, ok := p.root.tryGetNextItem()
	if !ok {
		return zero, false
	}
	return item, true
}

func verifyWeightedKeys[K comparable](weightedKeys []WeightedKey[K]) error {
	for _, weightedKey := range weightedKeys {
		if weightedKey.Weight <= 0 {
			return errWeightedKeyWeightMustBeGreaterThanZero
		}
	}
	return nil
}
