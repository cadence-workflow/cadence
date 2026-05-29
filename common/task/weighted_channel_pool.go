// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package task

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

const defaultIdleChannelTTLInSeconds = 3600

type (
	WeightedRoundRobinChannelPoolOptions struct {
		BufferSize              int
		IdleChannelTTLInSeconds int64
	}

	WeightedRoundRobinChannelPool[K comparable, V any] struct {
		sync.RWMutex
		status                  int32
		shutdownCh              chan struct{}
		shutdownWG              sync.WaitGroup
		bufferSize              int
		idleChannelTTLInSeconds int64
		logger                  log.Logger
		metricsScope            metrics.Scope
		timeSource              clock.TimeSource
		channelMap              map[K]weightedContainer[*TTLChannel[V]]

		// a snapshot of the channels to be used for the IWRR schedule
		iwrrSchedule atomic.Pointer[iwrrSchedule[*TTLChannel[V]]]
	}
)

func NewWeightedRoundRobinChannelPool[K comparable, V any](
	logger log.Logger,
	metricsScope metrics.Scope,
	timeSource clock.TimeSource,
	options WeightedRoundRobinChannelPoolOptions,
) *WeightedRoundRobinChannelPool[K, V] {
	wrr := &WeightedRoundRobinChannelPool[K, V]{
		bufferSize:              options.BufferSize,
		idleChannelTTLInSeconds: options.IdleChannelTTLInSeconds,
		logger:                  logger,
		metricsScope:            metricsScope,
		timeSource:              timeSource,
		channelMap:              make(map[K]weightedContainer[*TTLChannel[V]]),
		shutdownCh:              make(chan struct{}),
	}
	// Initialize with empty schedule
	schedule := newIWRRSchedule[K, *TTLChannel[V]](nil)
	wrr.iwrrSchedule.Store(schedule)
	return wrr
}

func (p *WeightedRoundRobinChannelPool[K, V]) Start() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	p.shutdownWG.Add(1)
	go p.cleanupLoop()

	p.logger.Info("Weighted round robin channel pool started.")
}

func (p *WeightedRoundRobinChannelPool[K, V]) Stop() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	close(p.shutdownCh)
	p.shutdownWG.Wait()

	p.logger.Info("Weighted round robin channel pool stopped.")
}

func (p *WeightedRoundRobinChannelPool[K, V]) cleanupLoop() {
	defer p.shutdownWG.Done()
	ticker := p.timeSource.NewTicker(time.Duration((p.idleChannelTTLInSeconds / 2)) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Chan():
			p.doCleanup()
		case <-p.shutdownCh:
			return
		}
	}
}

func (p *WeightedRoundRobinChannelPool[K, V]) doCleanup() {
	p.Lock()
	defer p.Unlock()
	var channelsToCleanup []K
	now := p.timeSource.Now()
	ttl := time.Duration(p.idleChannelTTLInSeconds) * time.Second
	for k, container := range p.channelMap {
		if container.item.ShouldCleanup(now, ttl) {
			channelsToCleanup = append(channelsToCleanup, k)
		}
	}

	for _, k := range channelsToCleanup {
		delete(p.channelMap, k)
	}

	if len(channelsToCleanup) > 0 {
		p.logger.Info("clean up idle channels", tag.Dynamic("channels", channelsToCleanup))
		p.updateScheduleLocked()
	}
}

func (p *WeightedRoundRobinChannelPool[K, V]) GetOrCreateChannel(key K, weight int) (chan V, func()) {
	p.RLock()
	if container, exists := p.channelMap[key]; exists && container.weight == weight {
		container.item.IncRef()
		container.item.UpdateLastWriteTime(p.timeSource.Now())
		p.RUnlock()
		return container.item.Chan(), container.item.DecRef
	}
	p.RUnlock()

	p.Lock()
	defer p.Unlock()
	if container, exists := p.channelMap[key]; exists {
		container.item.IncRef()
		container.item.UpdateLastWriteTime(p.timeSource.Now())
		if container.weight != weight {
			p.channelMap[key] = weightedContainer[*TTLChannel[V]]{
				item:   container.item,
				weight: weight,
			}
			p.updateScheduleLocked()
		}
		return container.item.Chan(), container.item.DecRef
	}

	c := NewTTLChannel[V](p.bufferSize)
	p.channelMap[key] = weightedContainer[*TTLChannel[V]]{
		item:   c,
		weight: weight,
	}
	c.IncRef()
	c.UpdateLastWriteTime(p.timeSource.Now())
	p.updateScheduleLocked()
	return c.Chan(), c.DecRef
}

func (p *WeightedRoundRobinChannelPool[K, V]) GetAllChannels() []chan V {
	p.RLock()
	defer p.RUnlock()
	allChannels := make([]chan V, 0, len(p.channelMap))
	for _, container := range p.channelMap {
		allChannels = append(allChannels, container.item.Chan())
	}
	return allChannels
}

func (p *WeightedRoundRobinChannelPool[K, V]) GetSchedule() Schedule[*TTLChannel[V]] {
	return p.iwrrSchedule.Load()
}

func (p *WeightedRoundRobinChannelPool[K, V]) updateScheduleLocked() {
	p.iwrrSchedule.Store(newIWRRSchedule(p.channelMap))

	// Update memory gauge - now only stores channel references once, not weight times
	memoryBytes := len(p.channelMap) * 16 // channel map entries
	p.metricsScope.UpdateGauge(metrics.WeightedChannelPoolSizeGauge, float64(memoryBytes))
}
