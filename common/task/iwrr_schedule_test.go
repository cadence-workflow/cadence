package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIWRRSchedule_Empty(t *testing.T) {
	schedule := newIWRRSchedule[int](nil)

	assert.Equal(t, 0, schedule.Len())

	iter := schedule.NewIterator()
	ch, ok := iter.TryNext()
	assert.False(t, ok)
	assert.Nil(t, ch)
}

func TestIWRRSchedule_SingleChannel(t *testing.T) {
	channels := []*weightedChannel[int]{
		{weight: 3, c: make(chan int, 10)},
	}

	schedule := newIWRRSchedule[int](channels)

	// Total length should be the weight
	assert.Equal(t, 3, schedule.Len())

	iter := schedule.NewIterator()

	// Should return the channel 3 times
	for i := 0; i < 3; i++ {
		ch, ok := iter.TryNext()
		assert.True(t, ok, "iteration %d should succeed", i)
		assert.Equal(t, channels[0].c, ch)
	}

	// Fourth call should return false (exhausted)
	ch, ok := iter.TryNext()
	assert.False(t, ok)
	assert.Nil(t, ch)
}

func TestIWRRSchedule_MultipleChannels_EqualWeights(t *testing.T) {
	channels := []*weightedChannel[int]{
		{weight: 2, c: make(chan int, 10)},
		{weight: 2, c: make(chan int, 10)},
		{weight: 2, c: make(chan int, 10)},
	}

	schedule := newIWRRSchedule[int](channels)

	assert.Equal(t, 6, schedule.Len())

	iter := schedule.NewIterator()

	// With equal weights, IWRR should interleave: [2, 1, 0, 2, 1, 0]
	// (highest index first in each round)
	expectedOrder := []int{2, 1, 0, 2, 1, 0}

	for i, expectedIdx := range expectedOrder {
		ch, ok := iter.TryNext()
		require.True(t, ok, "iteration %d should succeed", i)
		assert.Equal(t, channels[expectedIdx].c, ch, "iteration %d", i)
	}

	// Should be exhausted
	ch, ok := iter.TryNext()
	assert.False(t, ok)
	assert.Nil(t, ch)
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

	iter := schedule.NewIterator()

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
		ch, ok := iter.TryNext()
		require.True(t, ok, "iteration %d should succeed", i)
		assert.Equal(t, expectedCh, ch, "iteration %d", i)
	}

	// Should be exhausted
	ch, ok := iter.TryNext()
	assert.False(t, ok)
	assert.Nil(t, ch)
}

func TestIWRRSchedule_LargeWeights(t *testing.T) {
	channels := []*weightedChannel[int]{
		{weight: 100, c: make(chan int, 10)},
		{weight: 50, c: make(chan int, 10)},
		{weight: 25, c: make(chan int, 10)},
	}

	schedule := newIWRRSchedule[int](channels)

	assert.Equal(t, 175, schedule.Len())

	iter := schedule.NewIterator()

	// Count how many times each channel appears
	counts := make(map[chan int]int)

	for {
		ch, ok := iter.TryNext()
		if !ok {
			break
		}
		counts[ch]++
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

	iter := schedule.NewIterator()

	// Should only return channel with weight 3
	for i := 0; i < 3; i++ {
		ch, ok := iter.TryNext()
		require.True(t, ok, "iteration %d", i)
		assert.Equal(t, channels[1].c, ch)
	}

	// Should be exhausted
	ch, ok := iter.TryNext()
	assert.False(t, ok)
	assert.Nil(t, ch)
}

func TestIWRRSchedule_WeightedChannelFields(t *testing.T) {
	// Verify that the returned channel is correct
	// Note: The schedule only stores weight and c internally, and returns only the channel
	testChan := make(chan int, 10)
	channels := []*weightedChannel[int]{
		{
			weight: 10,
			c:      testChan,
		},
	}

	schedule := newIWRRSchedule[int](channels)

	iter := schedule.NewIterator()

	ch, ok := iter.TryNext()
	require.True(t, ok)
	assert.Equal(t, testChan, ch)
}

func TestIWRRSchedule_ExhaustedSchedule_MultipleCallsReturnFalse(t *testing.T) {
	channels := []*weightedChannel[int]{
		{weight: 1, c: make(chan int, 10)},
	}

	schedule := newIWRRSchedule[int](channels)

	iter := schedule.NewIterator()

	// Exhaust the iterator
	ch, ok := iter.TryNext()
	assert.True(t, ok)
	assert.NotNil(t, ch)

	// Multiple calls after exhaustion should all return false
	for i := 0; i < 5; i++ {
		ch, ok := iter.TryNext()
		assert.False(t, ok, "call %d after exhaustion", i)
		assert.Nil(t, ch, "call %d after exhaustion", i)
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

	iter := schedule.NewIterator()

	// IWRR pattern for [5, 3, 1]:
	// Round 4: [0]         (weight 5 > 4)
	// Round 3: [0]         (weight 5 > 3)
	// Round 2: [0, 1]      (weights 5,3 > 2)
	// Round 1: [0, 1]      (weights 5,3 > 1)
	// Round 0: [0, 1, 2]   (weights 5,3,1 > 0)
	// Result: [0, 0, 0, 1, 0, 1, 0, 1, 2]
	expectedPattern := []int{0, 0, 0, 1, 0, 1, 0, 1, 2}

	for i, expectedIdx := range expectedPattern {
		ch, ok := iter.TryNext()
		require.True(t, ok, "iteration %d", i)
		assert.Equal(t, channels[expectedIdx].c, ch, "iteration %d", i)
	}

	// Exhausted
	ch, ok := iter.TryNext()
	assert.False(t, ok)
	assert.Nil(t, ch)
}

func TestIWRRSchedule_StatelessSchedule_MultipleIterators(t *testing.T) {
	// Test that the schedule is stateless and can create multiple independent iterators
	c1 := make(chan int, 10)
	c2 := make(chan int, 10)
	channels := []*weightedChannel[int]{
		{weight: 2, c: c1},
		{weight: 1, c: c2},
	}

	schedule := newIWRRSchedule[int](channels)

	// IWRR for weights [2, 1]: [c1, c1, c2]
	// Create first iterator and consume partially
	iter1 := schedule.NewIterator()
	ch1, ok1 := iter1.TryNext()
	require.True(t, ok1)
	assert.Equal(t, c1, ch1)

	// Create second iterator - should start from the beginning
	iter2 := schedule.NewIterator()
	ch2, ok2 := iter2.TryNext()
	require.True(t, ok2)
	assert.Equal(t, c1, ch2, "second iterator should start from beginning")

	// First iterator should continue from where it left off
	ch1, ok1 = iter1.TryNext()
	require.True(t, ok1)
	assert.Equal(t, c1, ch1)

	// Second iterator should be independent and continue its own iteration
	ch2, ok2 = iter2.TryNext()
	require.True(t, ok2)
	assert.Equal(t, c1, ch2)
}
