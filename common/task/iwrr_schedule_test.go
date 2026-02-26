package task

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIWRRSchedule_Empty(t *testing.T) {
	schedule := newIWRRSchedule[int](nil)

	assert.Equal(t, 0, schedule.Len())

	wc, ok := schedule.Next()
	assert.False(t, ok)
	assert.Nil(t, wc)
}

func TestIWRRSchedule_SingleChannel(t *testing.T) {
	channels := []*weightedChannel[int]{
		{weight: 3, c: make(chan int, 10)},
	}

	schedule := newIWRRSchedule[int](channels)

	// Total length should be the weight
	assert.Equal(t, 3, schedule.Len())

	// Should return the channel 3 times
	for i := 0; i < 3; i++ {
		wc, ok := schedule.Next()
		assert.True(t, ok, "iteration %d should succeed", i)
		assert.Equal(t, channels[0].c, wc.c)
		assert.Equal(t, 3, wc.weight)
	}

	// Fourth call should return false (exhausted)
	wc, ok := schedule.Next()
	assert.False(t, ok)
	assert.Nil(t, wc)
}

func TestIWRRSchedule_MultipleChannels_EqualWeights(t *testing.T) {
	channels := []*weightedChannel[int]{
		{weight: 2, c: make(chan int, 10)},
		{weight: 2, c: make(chan int, 10)},
		{weight: 2, c: make(chan int, 10)},
	}

	schedule := newIWRRSchedule[int](channels)

	assert.Equal(t, 6, schedule.Len())

	// With equal weights, IWRR should interleave: [2, 1, 0, 2, 1, 0]
	// (highest index first in each round)
	expectedOrder := []int{2, 1, 0, 2, 1, 0}

	for i, expectedIdx := range expectedOrder {
		wc, ok := schedule.Next()
		require.True(t, ok, "iteration %d should succeed", i)
		assert.Equal(t, channels[expectedIdx].c, wc.c, "iteration %d", i)
	}

	// Should be exhausted
	wc, ok := schedule.Next()
	assert.False(t, ok)
	assert.Nil(t, wc)
}

func TestIWRRSchedule_MultipleChannels_DifferentWeights(t *testing.T) {
	// Create channels with weights [1, 2, 3]
	channels := []*weightedChannel[int]{
		{weight: 1, c: make(chan int, 10)},
		{weight: 2, c: make(chan int, 10)},
		{weight: 3, c: make(chan int, 10)},
	}

	schedule := newIWRRSchedule[int](channels)

	assert.Equal(t, 6, schedule.Len())

	// IWRR with weights [1, 2, 3] processes rounds from 2 down to 0:
	// Round 2: channels with weight > 2 → channel 2 (weight 3)
	// Round 1: channels with weight > 1 → channels 2, 1 (weights 3, 2)
	// Round 0: channels with weight > 0 → channels 2, 1, 0 (weights 3, 2, 1)
	// Result: [2, 2, 1, 2, 1, 0]
	expectedSequence := []chan int{
		channels[2].c, // round 2: weight 3
		channels[2].c, // round 1: weight 3
		channels[1].c, // round 1: weight 2
		channels[2].c, // round 0: weight 3
		channels[1].c, // round 0: weight 2
		channels[0].c, // round 0: weight 1
	}

	for i, expectedCh := range expectedSequence {
		wc, ok := schedule.Next()
		require.True(t, ok, "iteration %d should succeed", i)
		assert.Equal(t, expectedCh, wc.c, "iteration %d", i)
	}

	// Should be exhausted
	wc, ok := schedule.Next()
	assert.False(t, ok)
	assert.Nil(t, wc)
}

func TestIWRRSchedule_LargeWeights(t *testing.T) {
	channels := []*weightedChannel[int]{
		{weight: 100, c: make(chan int, 10)},
		{weight: 50, c: make(chan int, 10)},
		{weight: 25, c: make(chan int, 10)},
	}

	schedule := newIWRRSchedule[int](channels)

	assert.Equal(t, 175, schedule.Len())

	// Count how many times each channel appears
	counts := make(map[chan int]int)

	for {
		wc, ok := schedule.Next()
		if !ok {
			break
		}
		counts[wc.c]++
	}

	assert.Equal(t, 100, counts[channels[0].c], "channel 0 should appear 100 times")
	assert.Equal(t, 50, counts[channels[1].c], "channel 1 should appear 50 times")
	assert.Equal(t, 25, counts[channels[2].c], "channel 2 should appear 25 times")
}

func TestIWRRSchedule_ChannelWithZeroWeight(t *testing.T) {
	channels := []*weightedChannel[int]{
		{weight: 0, c: make(chan int, 10)},
		{weight: 3, c: make(chan int, 10)},
	}

	schedule := newIWRRSchedule[int](channels)

	// Total length should only count non-zero weights
	assert.Equal(t, 3, schedule.Len())

	// Should only return channel with weight 3
	for i := 0; i < 3; i++ {
		wc, ok := schedule.Next()
		require.True(t, ok, "iteration %d", i)
		assert.Equal(t, channels[1].c, wc.c)
		assert.Equal(t, 3, wc.weight)
	}

	// Should be exhausted
	wc, ok := schedule.Next()
	assert.False(t, ok)
	assert.Nil(t, wc)
}

func TestIWRRSchedule_WeightedChannelFields(t *testing.T) {
	// Verify that the returned weightedChannel has all fields accessible
	refCount := atomic.Int32{}
	refCount.Store(5)
	lastWrite := atomic.Int64{}
	lastWrite.Store(12345)

	channels := []*weightedChannel[int]{
		{
			weight:        10,
			c:             make(chan int, 10),
			refCount:      refCount,
			lastWriteTime: lastWrite,
		},
	}

	schedule := newIWRRSchedule[int](channels)

	wc, ok := schedule.Next()
	require.True(t, ok)
	assert.Equal(t, 10, wc.weight)
	assert.Equal(t, channels[0].c, wc.c)
	// Note: refCount and lastWriteTime are copied by value in the snapshot
}

func TestIWRRSchedule_ExhaustedSchedule_MultipleCallsReturnFalse(t *testing.T) {
	channels := []*weightedChannel[int]{
		{weight: 1, c: make(chan int, 10)},
	}

	schedule := newIWRRSchedule[int](channels)

	// Exhaust the schedule
	wc, ok := schedule.Next()
	assert.True(t, ok)
	assert.NotNil(t, wc)

	// Multiple calls after exhaustion should all return false
	for i := 0; i < 5; i++ {
		wc, ok := schedule.Next()
		assert.False(t, ok, "call %d after exhaustion", i)
		assert.Nil(t, wc, "call %d after exhaustion", i)
	}
}

func TestIWRRSchedule_Ordering_Weights_5_3_1(t *testing.T) {
	// Test case from task pool tests: weights [5, 3, 1]
	channels := []*weightedChannel[int]{
		{weight: 5, c: make(chan int, 10)},
		{weight: 3, c: make(chan int, 10)},
		{weight: 1, c: make(chan int, 10)},
	}

	schedule := newIWRRSchedule[int](channels)

	assert.Equal(t, 9, schedule.Len())

	// IWRR pattern for [5, 3, 1]:
	// Round 4: [0]         (weight 5 > 4)
	// Round 3: [0]         (weight 5 > 3)
	// Round 2: [0, 1]      (weights 5,3 > 2)
	// Round 1: [0, 1]      (weights 5,3 > 1)
	// Round 0: [0, 1, 2]   (weights 5,3,1 > 0)
	// Result: [0, 0, 0, 1, 0, 1, 0, 1, 2]
	expectedPattern := []int{0, 0, 0, 1, 0, 1, 0, 1, 2}

	for i, expectedIdx := range expectedPattern {
		wc, ok := schedule.Next()
		require.True(t, ok, "iteration %d", i)
		assert.Equal(t, channels[expectedIdx].c, wc.c, "iteration %d", i)
	}

	// Exhausted
	wc, ok := schedule.Next()
	assert.False(t, ok)
	assert.Nil(t, wc)
}
