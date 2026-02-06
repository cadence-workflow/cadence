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
	"sort"
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
	weightedChannels[V any] []*weightedChannel[V]

	weightedChannel[V any] struct {
		weight        int
		c             chan V
		refCount      atomic.Int32
		lastWriteTime atomic.Int64
	}

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
		channelMap              map[K]*weightedChannel[V]
		// precalculated / flattened task chan schedule according to weight
		// e.g. if
		// ChannelKeyToWeight has the following mapping
		//  0 -> 5
		//  1 -> 3
		//  2 -> 2
		//  3 -> 1
		// then iwrrChannels will contain chan [0, 0, 0, 1, 0, 1, 2, 0, 1, 2, 3] (ID-ed by channel key)
		// This implementation uses interleaved weighted round robin schedule instead of classic weighted round robin schedule
		// ref: https://en.wikipedia.org/wiki/Weighted_round_robin#Interleaved_WRR
		iwrrSchedule atomic.Value // []*weightedChannel[V]
	}
)

func NewWeightedRoundRobinChannelPool[K comparable, V any](
	logger log.Logger,
	metricsScope metrics.Scope,
	timeSource clock.TimeSource,
	options WeightedRoundRobinChannelPoolOptions,
) *WeightedRoundRobinChannelPool[K, V] {
	return &WeightedRoundRobinChannelPool[K, V]{
		bufferSize:              options.BufferSize,
		idleChannelTTLInSeconds: options.IdleChannelTTLInSeconds,
		logger:                  logger,
		metricsScope:            metricsScope,
		timeSource:              timeSource,
		channelMap:              make(map[K]*weightedChannel[V]),
		shutdownCh:              make(chan struct{}),
	}
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
	now := p.timeSource.Now().Unix()
	for k, v := range p.channelMap {
		if now-v.lastWriteTime.Load() > p.idleChannelTTLInSeconds && len(v.c) == 0 && v.refCount.Load() == 0 {
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
	if v := p.channelMap[key]; v != nil && v.weight == weight {
		v.refCount.Add(1)
		v.lastWriteTime.Store(p.timeSource.Now().Unix())
		p.RUnlock()
		return v.c, func() {
			v.refCount.Add(-1)
		}
	}
	p.RUnlock()

	p.Lock()
	defer p.Unlock()
	if v := p.channelMap[key]; v != nil {
		v.refCount.Add(1)
		v.lastWriteTime.Store(p.timeSource.Now().Unix())
		if v.weight != weight {
			v.weight = weight
			p.updateScheduleLocked()
		}
		return v.c, func() {
			v.refCount.Add(-1)
		}
	}

	v := &weightedChannel[V]{
		weight: weight,
		c:      make(chan V, p.bufferSize),
	}
	p.channelMap[key] = v
	v.refCount.Add(1)
	v.lastWriteTime.Store(p.timeSource.Now().Unix())
	p.updateScheduleLocked()
	return v.c, func() {
		v.refCount.Add(-1)
	}
}

func (p *WeightedRoundRobinChannelPool[K, V]) GetAllChannels() []chan V {
	p.RLock()
	defer p.RUnlock()
	allChannels := make([]chan V, 0, len(p.channelMap))
	for _, v := range p.channelMap {
		allChannels = append(allChannels, v.c)
	}
	return allChannels
}

func (p *WeightedRoundRobinChannelPool[K, V]) GetSchedule() []chan V {
	return p.iwrrSchedule.Load().([]chan V)
}

func (p *WeightedRoundRobinChannelPool[K, V]) updateScheduleLocked() {
	totalWeight := 0
	orderedChannels := make(weightedChannels[V], 0, len(p.channelMap))
	for _, v := range p.channelMap {
		totalWeight += v.weight
		orderedChannels = append(orderedChannels, v)
	}
	sort.Sort(orderedChannels)

	iwrrSchedule := make([]chan V, 0, totalWeight)
	if totalWeight == 0 {
		p.iwrrSchedule.Store(iwrrSchedule)
		return
	}

	maxWeight := orderedChannels[len(orderedChannels)-1].weight
	for round := maxWeight - 1; round >= 0; round-- {
		for i := len(orderedChannels) - 1; i >= 0 && orderedChannels[i].weight > round; i-- {
			iwrrSchedule = append(iwrrSchedule, orderedChannels[i].c)
		}
	}
	p.iwrrSchedule.Store(iwrrSchedule)

	p.metricsScope.UpdateGauge(metrics.WeightedChannelPoolSizeGauge, float64((len(p.channelMap)+len(iwrrSchedule))*28)) // 28 bytes per weighted channel struct
}

func (w weightedChannels[V]) Len() int {
	return len(w)
}

func (w weightedChannels[V]) Less(i, j int) bool {
	return w[i].weight < w[j].weight
}

func (w weightedChannels[V]) Swap(i, j int) {
	w[i], w[j] = w[j], w[i]
}
