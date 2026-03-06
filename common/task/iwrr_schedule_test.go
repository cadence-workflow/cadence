package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testWeightedItem is a simple implementation for testing
type testWeightedItem struct {
	id int
}

// toWeightedMap converts a map of items with weights to a map of weightedContainers
func toWeightedMap(items map[int]*testWeightedItem, weights map[int]int) map[int]weightedContainer[*testWeightedItem] {
	result := make(map[int]weightedContainer[*testWeightedItem])
	for k, item := range items {
		result[k] = weightedContainer[*testWeightedItem]{
			item:   item,
			weight: weights[k],
		}
	}
	return result
}

func TestIWRRSchedule_Empty(t *testing.T) {
	schedule := newIWRRSchedule[int, *testWeightedItem](nil)

	assert.Equal(t, 0, schedule.Len())

	iter := schedule.NewIterator()
	item, ok := iter.TryNext()
	assert.False(t, ok)
	assert.Nil(t, item)
}

func TestIWRRSchedule_SingleChannel(t *testing.T) {
	items := map[int]*testWeightedItem{
		0: {id: 0},
	}
	weights := map[int]int{
		0: 3,
	}

	schedule := newIWRRSchedule[int, *testWeightedItem](toWeightedMap(items, weights))

	// Total length should be the weight
	assert.Equal(t, 3, schedule.Len())

	iter := schedule.NewIterator()

	// Should return the item 3 times
	for i := 0; i < 3; i++ {
		item, ok := iter.TryNext()
		assert.True(t, ok, "iteration %d should succeed", i)
		assert.Equal(t, items[0], item)
	}

	// Fourth call should return false (exhausted)
	item, ok := iter.TryNext()
	assert.False(t, ok)
	assert.Nil(t, item)
}

func TestIWRRSchedule_MultipleChannels_EqualWeights(t *testing.T) {
	items := map[int]*testWeightedItem{
		0: {id: 0},
		1: {id: 1},
		2: {id: 2},
	}
	weights := map[int]int{
		0: 2,
		1: 2,
		2: 2,
	}

	schedule := newIWRRSchedule[int, *testWeightedItem](toWeightedMap(items, weights))

	assert.Equal(t, 6, schedule.Len())

	iter := schedule.NewIterator()

	// With equal weights, each item should appear equal number of times
	// Note: the exact order depends on map iteration order
	counts := make(map[*testWeightedItem]int)
	for {
		item, ok := iter.TryNext()
		if !ok {
			break
		}
		counts[item]++
	}

	assert.Equal(t, 2, counts[items[0]], "item 0 should appear 2 times")
	assert.Equal(t, 2, counts[items[1]], "item 1 should appear 2 times")
	assert.Equal(t, 2, counts[items[2]], "item 2 should appear 2 times")
}

func TestIWRRSchedule_MultipleChannels_DifferentWeights(t *testing.T) {
	// Create items with weights [1, 2, 3]
	items := map[int]*testWeightedItem{
		0: {id: 0},
		1: {id: 1},
		2: {id: 2},
	}
	weights := map[int]int{
		0: 1,
		1: 2,
		2: 3,
	}

	schedule := newIWRRSchedule[int, *testWeightedItem](toWeightedMap(items, weights))

	assert.Equal(t, 6, schedule.Len())

	iter := schedule.NewIterator()

	// IWRR with weights [1, 2, 3] processes rounds from 2 down to 0:
	// Round 2: items with weight > 2 → item 2 (weight 3)
	// Round 1: items with weight > 1 → items 2, 1 (weights 3, 2)
	// Round 0: items with weight > 0 → items 2, 1, 0 (weights 3, 2, 1)
	// Result: [2, 2, 1, 2, 1, 0]
	expectedSequence := []*testWeightedItem{
		items[2], // round 2: weight 3
		items[2], // round 1: weight 3
		items[1], // round 1: weight 2
		items[2], // round 0: weight 3
		items[1], // round 0: weight 2
		items[0], // round 0: weight 1
	}

	for i, expected := range expectedSequence {
		item, ok := iter.TryNext()
		require.True(t, ok, "iteration %d should succeed", i)
		assert.Equal(t, expected, item, "iteration %d", i)
	}

	// Should be exhausted
	item, ok := iter.TryNext()
	assert.False(t, ok)
	assert.Nil(t, item)
}

func TestIWRRSchedule_LargeWeights(t *testing.T) {
	items := map[int]*testWeightedItem{
		0: {id: 0},
		1: {id: 1},
		2: {id: 2},
	}
	weights := map[int]int{
		0: 100,
		1: 50,
		2: 25,
	}

	schedule := newIWRRSchedule[int, *testWeightedItem](toWeightedMap(items, weights))

	assert.Equal(t, 175, schedule.Len())

	iter := schedule.NewIterator()

	// IWRR pattern for [100, 50, 25]:
	// Rounds 99-50 (50 rounds): only item 0 (weight 100 > round)
	// Rounds 49-25 (25 rounds): items 0, 1 (weights 100, 50 > round)
	// Rounds 24-0 (25 rounds): items 0, 1, 2 (all weights > round)
	var expectedSequence []*testWeightedItem

	// First 50 rounds: only item 0
	for i := 0; i < 50; i++ {
		expectedSequence = append(expectedSequence, items[0])
	}

	// Next 25 rounds: items 0, 1
	for i := 0; i < 25; i++ {
		expectedSequence = append(expectedSequence, items[0], items[1])
	}

	// Last 25 rounds: items 0, 1, 2
	for i := 0; i < 25; i++ {
		expectedSequence = append(expectedSequence, items[0], items[1], items[2])
	}

	// Verify the sequence
	for i, expected := range expectedSequence {
		item, ok := iter.TryNext()
		require.True(t, ok, "iteration %d should succeed", i)
		assert.Equal(t, expected, item, "iteration %d", i)
	}

	// Should be exhausted
	item, ok := iter.TryNext()
	assert.False(t, ok)
	assert.Nil(t, item)
}

func TestIWRRSchedule_ChannelWithZeroWeight(t *testing.T) {
	items := map[int]*testWeightedItem{
		0: {id: 0},
		1: {id: 1},
	}
	weights := map[int]int{
		0: 0,
		1: 3,
	}

	schedule := newIWRRSchedule[int, *testWeightedItem](toWeightedMap(items, weights))

	// Total length should only count non-zero weights
	assert.Equal(t, 3, schedule.Len())

	iter := schedule.NewIterator()

	// Should only return item with weight 3
	for i := 0; i < 3; i++ {
		item, ok := iter.TryNext()
		require.True(t, ok, "iteration %d", i)
		assert.Equal(t, items[1], item)
	}

	// Should be exhausted
	item, ok := iter.TryNext()
	assert.False(t, ok)
	assert.Nil(t, item)
}

func TestIWRRSchedule_WeightedChannelFields(t *testing.T) {
	// Verify that the returned item is correct
	testItem := &testWeightedItem{
		id: 0,
	}
	items := map[int]*testWeightedItem{
		0: testItem,
	}
	weights := map[int]int{
		0: 10,
	}

	schedule := newIWRRSchedule[int, *testWeightedItem](toWeightedMap(items, weights))

	iter := schedule.NewIterator()

	item, ok := iter.TryNext()
	require.True(t, ok)
	assert.Equal(t, testItem, item)
}

func TestIWRRSchedule_ExhaustedSchedule_MultipleCallsReturnFalse(t *testing.T) {
	items := map[int]*testWeightedItem{
		0: {id: 0},
	}
	weights := map[int]int{
		0: 1,
	}

	schedule := newIWRRSchedule[int, *testWeightedItem](toWeightedMap(items, weights))

	iter := schedule.NewIterator()

	// Exhaust the iterator
	item, ok := iter.TryNext()
	assert.True(t, ok)
	assert.NotNil(t, item)

	// Multiple calls after exhaustion should all return false
	for i := 0; i < 5; i++ {
		item, ok := iter.TryNext()
		assert.False(t, ok, "call %d after exhaustion", i)
		assert.Nil(t, item, "call %d after exhaustion", i)
	}
}

func TestIWRRSchedule_Ordering_Weights_5_3_1(t *testing.T) {
	// Test case from task pool tests: weights [5, 3, 1]
	items := map[int]*testWeightedItem{
		0: {id: 0},
		1: {id: 1},
		2: {id: 2},
	}
	weights := map[int]int{
		0: 5,
		1: 3,
		2: 1,
	}

	schedule := newIWRRSchedule[int, *testWeightedItem](toWeightedMap(items, weights))

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
		item, ok := iter.TryNext()
		require.True(t, ok, "iteration %d", i)
		assert.Equal(t, items[expectedIdx], item, "iteration %d", i)
	}

	// Exhausted
	item, ok := iter.TryNext()
	assert.False(t, ok)
	assert.Nil(t, item)
}

func TestIWRRSchedule_StatelessSchedule_MultipleIterators(t *testing.T) {
	// Test that the schedule is stateless and can create multiple independent iterators
	item1 := &testWeightedItem{id: 0}
	item2 := &testWeightedItem{id: 1}
	items := map[int]*testWeightedItem{
		0: item1,
		1: item2,
	}
	weights := map[int]int{
		0: 2,
		1: 1,
	}

	schedule := newIWRRSchedule[int, *testWeightedItem](toWeightedMap(items, weights))

	// IWRR for weights [2, 1]: [item1, item1, item2]
	// Create first iterator and consume partially
	iter1 := schedule.NewIterator()
	i1, ok1 := iter1.TryNext()
	require.True(t, ok1)
	assert.Equal(t, item1, i1)

	// Create second iterator - should start from the beginning
	iter2 := schedule.NewIterator()
	i2, ok2 := iter2.TryNext()
	require.True(t, ok2)
	assert.Equal(t, item1, i2, "second iterator should start from beginning")

	// First iterator should continue from where it left off
	i1, ok1 = iter1.TryNext()
	require.True(t, ok1)
	assert.Equal(t, item1, i1)

	// Second iterator should be independent and continue its own iteration
	i2, ok2 = iter2.TryNext()
	require.True(t, ok2)
	assert.Equal(t, item1, i2)
}
