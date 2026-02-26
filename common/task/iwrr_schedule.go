package task

import "sort"

// iwrrSchedule implements Schedule using an efficient interleaved weighted round-robin algorithm
// It computes the next channel on-demand without materializing the entire schedule
type iwrrSchedule[V any] struct {
	// Snapshot of channels sorted by weight (ascending)
	channels weightedChannels[V]
	// Maximum weight among all channels
	maxWeight int
	// Total virtual length (sum of all weights)
	totalLen int
	// Current iteration state
	currentRound int // Current round (maxWeight-1 down to 0)
	currentIndex int // Index within current round's qualifying channels
}

// newIWRRSchedule creates a new IWRR schedule from a snapshot of weighted channels
func newIWRRSchedule[V any](channels weightedChannels[V]) Schedule[*weightedChannel[V]] {
	if len(channels) == 0 {
		return &iwrrSchedule[V]{}
	}

	// Make a copy to snapshot the channels
	channelsCopy := make(weightedChannels[V], len(channels))
	copy(channelsCopy, channels)

	sort.Sort(channelsCopy)

	maxWeight := channelsCopy[len(channelsCopy)-1].weight
	totalLen := 0
	for _, ch := range channelsCopy {
		totalLen += ch.weight
	}

	return &iwrrSchedule[V]{
		channels:     channelsCopy,
		maxWeight:    maxWeight,
		totalLen:     totalLen,
		currentRound: maxWeight - 1,
		currentIndex: len(channelsCopy) - 1,
	}
}

// Next returns the next weighted channel in the IWRR schedule
// The algorithm processes rounds from maxWeight-1 down to 0
// In each round r, channels with weight > r are included
// Returns false when the schedule is exhausted (all rounds completed)
func (s *iwrrSchedule[V]) Next() (*weightedChannel[V], bool) {
	if len(s.channels) == 0 {
		return nil, false
	}

	// Find the next qualifying channel
	for {
		// Find channels that qualify for current round (weight > round)
		// We iterate from highest weight to lowest
		for s.currentIndex >= 0 && s.channels[s.currentIndex].weight > s.currentRound {
			ch := s.channels[s.currentIndex]
			s.currentIndex--
			return ch, true
		}

		// Move to next round
		s.currentRound--
		if s.currentRound < 0 {
			// Schedule exhausted - all rounds completed
			return nil, false
		}
		s.currentIndex = len(s.channels) - 1
	}
}

// Len returns the total virtual length of the schedule
func (s *iwrrSchedule[V]) Len() int {
	return s.totalLen
}
