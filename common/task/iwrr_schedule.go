package task

import "slices"

var _ Schedule[any] = &iwrrSchedule[any]{}

// weightedContainer is a container for an item with a weight
type weightedContainer[V any] struct {
	weight int
	item   V
}

// iwrrSchedule implements Schedule using an efficient interleaved weighted round-robin algorithm
// ref: https://en.wikipedia.org/wiki/Weighted_round_robin#Interleaved_WRR
// It is stateless and creates iterators on demand
type iwrrSchedule[V any] struct {
	// Snapshot of items sorted by weight (ascending)
	items []weightedContainer[V]
	// Maximum weight among all items
	maxWeight int
	// Total virtual length (sum of all weights)
	totalLen int
}

// iwrrIterator is a stateful iterator that tracks position in the schedule
type iwrrIterator[V any] struct {
	schedule     *iwrrSchedule[V]
	currentRound int // Current round (maxWeight-1 down to 0)
	currentIndex int // Index within current round's qualifying items
}

// newIWRRSchedule creates a new IWRR schedule from a snapshot of weighted containers
// Items with weight <= 0 are ignored
func newIWRRSchedule[K comparable, V any](items map[K]weightedContainer[V]) *iwrrSchedule[V] {
	if len(items) == 0 {
		return &iwrrSchedule[V]{}
	}

	// Filter out items with weight <= 0 and copy to slice
	itemsCopy := make([]weightedContainer[V], 0, len(items))
	totalLen := 0
	for _, container := range items {
		if container.weight > 0 {
			itemsCopy = append(itemsCopy, container)
			totalLen += container.weight
		}
	}

	// Return empty schedule if no valid items
	if len(itemsCopy) == 0 {
		return &iwrrSchedule[V]{}
	}

	// Sort by weight (ascending)
	slices.SortFunc(itemsCopy, func(a, b weightedContainer[V]) int {
		return a.weight - b.weight
	})

	maxWeight := itemsCopy[len(itemsCopy)-1].weight

	return &iwrrSchedule[V]{
		items:     itemsCopy,
		maxWeight: maxWeight,
		totalLen:  totalLen,
	}
}

// NewIterator creates a new stateful iterator for this schedule
func (s *iwrrSchedule[V]) NewIterator() Iterator[V] {
	if len(s.items) == 0 {
		return &iwrrIterator[V]{schedule: s}
	}
	return &iwrrIterator[V]{
		schedule:     s,
		currentRound: s.maxWeight - 1,
		currentIndex: len(s.items) - 1,
	}
}

// Len returns the total virtual length of the schedule
func (s *iwrrSchedule[V]) Len() int {
	return s.totalLen
}

// TryNext returns the next item in the IWRR iteration
// The algorithm processes rounds from maxWeight-1 down to 0
// In each round r, items with weight > r are included
// Returns false when the iteration is exhausted (all rounds completed)
func (it *iwrrIterator[V]) TryNext() (zero V, ok bool) {
	if it.schedule == nil || len(it.schedule.items) == 0 {
		return
	}

	// Find the next qualifying item
	for it.currentRound >= 0 {
		// Find items that qualify for current round (weight > round)
		// We iterate from highest weight to lowest
		for it.currentIndex >= 0 && it.schedule.items[it.currentIndex].weight > it.currentRound {
			item := it.schedule.items[it.currentIndex].item
			it.currentIndex--
			return item, true
		}

		// Move to next round
		it.currentRound--
		it.currentIndex = len(it.schedule.items) - 1
	}
	return
}
