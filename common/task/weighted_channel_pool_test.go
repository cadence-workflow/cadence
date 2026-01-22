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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

func TestGetOrCreateChannel(t *testing.T) {
	timeSource := clock.NewMockedTimeSource()
	pool := NewWeightedRoundRobinChannelPool[string, int](
		testlogger.New(t),
		metrics.NoopScope,
		timeSource,
		WeightedRoundRobinChannelPoolOptions{
			BufferSize:              1000,
			IdleChannelTTLInSeconds: 10,
		},
	)

	// First, verify that the method returns the same channel if the key and weight are the same
	c1, releaseFn1 := pool.GetOrCreateChannel("k1", 1)
	defer releaseFn1()
	c2, releaseFn2 := pool.GetOrCreateChannel("k1", 1)
	defer releaseFn2()
	assert.Equal(t, c1, c2)

	// Next, verify that the methods returns the same channel if the key is the same but weight is different
	c3, releaseFn3 := pool.GetOrCreateChannel("k1", 2)
	defer releaseFn3()
	assert.Equal(t, c1, c3)
}

func TestGetOrCreateChannelConcurrent(t *testing.T) {
	timeSource := clock.NewMockedTimeSource()
	pool := NewWeightedRoundRobinChannelPool[string, int](
		testlogger.New(t),
		metrics.NoopScope,
		timeSource,
		WeightedRoundRobinChannelPoolOptions{
			BufferSize:              1000,
			IdleChannelTTLInSeconds: 10,
		},
	)

	var wg sync.WaitGroup
	var chMap sync.Map
	wg.Add(15)

	for i := 0; i < 5; i++ {
		go func(i int) {
			defer wg.Done()
			c, releaseFn := pool.GetOrCreateChannel("k1", i+1)
			defer releaseFn()
			chMap.Store("k1", c)
		}(i)
		go func(i int) {
			defer wg.Done()
			c, releaseFn := pool.GetOrCreateChannel("k2", i+1)
			defer releaseFn()
			chMap.Store("k2", c)
		}(i)
		go func(i int) {
			defer wg.Done()
			c, releaseFn := pool.GetOrCreateChannel("k3", i+1)
			defer releaseFn()
			chMap.Store("k3", c)
		}(i)
	}
	wg.Wait()

	chs := pool.GetAllChannels()
	assert.Len(t, chs, 3)
	ch1, _ := chMap.Load("k1")
	ch2, _ := chMap.Load("k2")
	ch3, _ := chMap.Load("k3")
	expectedChs := []chan int{ch1.(chan int), ch2.(chan int), ch3.(chan int)}
	assert.ElementsMatch(t, expectedChs, chs)
}

func TestGetSchedule(t *testing.T) {
	timeSource := clock.NewMockedTimeSource()
	pool := NewWeightedRoundRobinChannelPool[string, int](
		testlogger.New(t),
		metrics.NoopScope,
		timeSource,
		WeightedRoundRobinChannelPoolOptions{
			BufferSize:              1000,
			IdleChannelTTLInSeconds: 10,
		},
	)

	c1, releaseFn1 := pool.GetOrCreateChannel("k1", 1)
	defer releaseFn1()
	c2, releaseFn2 := pool.GetOrCreateChannel("k2", 2)
	defer releaseFn2()
	c3, releaseFn3 := pool.GetOrCreateChannel("k3", 3)
	defer releaseFn3()

	schedule := pool.GetSchedule()
	assert.Len(t, schedule, 6)

	assert.Equal(t, c3, schedule[0])
	assert.Equal(t, c3, schedule[1])
	assert.Equal(t, c2, schedule[2])
	assert.Equal(t, c3, schedule[3])
	assert.Equal(t, c2, schedule[4])
	assert.Equal(t, c1, schedule[5])

	c4, releaseFn4 := pool.GetOrCreateChannel("k2", 4)
	defer releaseFn4()
	assert.Equal(t, c2, c4)
	schedule = pool.GetSchedule()
	assert.Len(t, schedule, 8)

	assert.Equal(t, c2, schedule[0])
	assert.Equal(t, c2, schedule[1])
	assert.Equal(t, c3, schedule[2])
	assert.Equal(t, c2, schedule[3])
	assert.Equal(t, c3, schedule[4])
	assert.Equal(t, c2, schedule[5])
	assert.Equal(t, c3, schedule[6])
	assert.Equal(t, c1, schedule[7])
}

func TestCleanup(t *testing.T) {
	timeSource := clock.NewRealTimeSource()
	pool := NewWeightedRoundRobinChannelPool[string, int](
		testlogger.New(t),
		metrics.NoopScope,
		timeSource,
		WeightedRoundRobinChannelPoolOptions{
			BufferSize:              1000,
			IdleChannelTTLInSeconds: 2,
		},
	)
	pool.Start()
	defer pool.Stop()

	// First, verify that the method returns the same channel if the key and weight are the same
	_, releaseFn1 := pool.GetOrCreateChannel("k1", 1)
	ch2, releaseFn2 := pool.GetOrCreateChannel("k2", 1)
	ch3, releaseFn3 := pool.GetOrCreateChannel("k3", 1)
	ch3 <- 1

	assert.Len(t, pool.GetAllChannels(), 3)
	releaseFn1()
	releaseFn3()
	time.Sleep(time.Second * 4)
	// only c1 is deleted
	chs := pool.GetAllChannels()
	assert.ElementsMatch(t, chs, []chan int{ch2, ch3})

	releaseFn2()
}
