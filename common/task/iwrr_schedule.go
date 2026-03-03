package task

import "slices"

var _ Schedule[chan any] = &iwrrSchedule[any]{}

// scheduledChannel is a snapshot of the essential fields from weightedChannel
// Used internally by iwrrSchedule to avoid race conditions with the original weightedChannel
type scheduledChannel[V any] struct {
	weight int
	c      chan V
}

// iwrrSchedule implements Schedule using an efficient interleaved weighted round-robin algorithm
// ref: https://en.wikipedia.org/wiki/Weighted_round_robin#Interleaved_WRR
// It is stateless and creates iterators on demand
type iwrrSchedule[V any] struct {
	// Snapshot of channels sorted by weight (ascending)
	channels []scheduledChannel[V]
	// Maximum weight among all channels
	maxWeight int
	// Total virtual length (sum of all weights)
	totalLen int
}

// iwrrIterator is a stateful iterator that tracks position in the schedule
type iwrrIterator[V any] struct {
	schedule     *iwrrSchedule[V]
	currentRound int // Current round (maxWeight-1 down to 0)
	currentIndex int // Index within current round's qualifying channels
}

// newIWRRSchedule creates a new IWRR schedule from a snapshot of weighted channels
// Channels with weight <= 0 are ignored
func newIWRRSchedule[V any](channels weightedChannels[V]) *iwrrSchedule[V] {
	if len(channels) == 0 {
		return &iwrrSchedule[V]{}
	}

	// Filter out channels with weight <= 0 and copy only the fields we need
	// This avoids race conditions with the original weightedChannel objects
	channelsCopy := make([]scheduledChannel[V], 0, len(channels))
	totalLen := 0
	for _, ch := range channels {
		if ch.weight > 0 {
			channelsCopy = append(channelsCopy, scheduledChannel[V]{
				weight: ch.weight,
				c:      ch.c,
			})
			totalLen += ch.weight
		}
	}

	// Return empty schedule if no valid channels
	if len(channelsCopy) == 0 {
		return &iwrrSchedule[V]{}
	}

	// Sort by weight (ascending)
	slices.SortFunc(channelsCopy, func(a, b scheduledChannel[V]) int {
		return a.weight - b.weight
	})

	maxWeight := channelsCopy[len(channelsCopy)-1].weight

	return &iwrrSchedule[V]{
		channels:  channelsCopy,
		maxWeight: maxWeight,
		totalLen:  totalLen,
	}
}

// NewIterator creates a new stateful iterator for this schedule
func (s *iwrrSchedule[V]) NewIterator() Iterator[chan V] {
	if len(s.channels) == 0 {
		return &iwrrIterator[V]{schedule: s}
	}
	return &iwrrIterator[V]{
		schedule:     s,
		currentRound: s.maxWeight - 1,
		currentIndex: len(s.channels) - 1,
	}
}

// Len returns the total virtual length of the schedule
func (s *iwrrSchedule[V]) Len() int {
	return s.totalLen
}

// TryNext returns the next channel in the IWRR iteration
// The algorithm processes rounds from maxWeight-1 down to 0
// In each round r, channels with weight > r are included
// Returns false when the iteration is exhausted (all rounds completed)
func (it *iwrrIterator[V]) TryNext() (chan V, bool) {
	if it.schedule == nil || len(it.schedule.channels) == 0 {
		return nil, false
	}

	// Find the next qualifying channel
	for it.currentRound >= 0 {
		// Find channels that qualify for current round (weight > round)
		// We iterate from highest weight to lowest
		for it.currentIndex >= 0 && it.schedule.channels[it.currentIndex].weight > it.currentRound {
			ch := it.schedule.channels[it.currentIndex].c
			it.currentIndex--
			return ch, true
		}

		// Move to next round
		it.currentRound--
		it.currentIndex = len(it.schedule.channels) - 1
	}
	return nil, false
}
