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

package tasklist

import (
	"math"
	"slices"

	"golang.org/x/exp/maps"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
)

type (
	isolationBalancer struct {
		timeSource clock.TimeSource
		scope      metrics.Scope
		config     *config.TaskListConfig
		groupState map[string]*isolationGroupState
	}

	isolationGroupState struct {
		underLoad             clock.Sustain
		overLoad              clock.Sustain
		noPollers             clock.Sustain
		hasPollers            clock.Sustain
		canAssignToPartitions bool
	}
)

func newIsolationBalancer(timeSource clock.TimeSource, scope metrics.Scope, config *config.TaskListConfig) *isolationBalancer {
	return &isolationBalancer{
		timeSource: timeSource,
		scope:      scope,
		config:     config,
		groupState: make(map[string]*isolationGroupState),
	}
}

func (i *isolationBalancer) adjustWritePartitions(metrics *aggregatePartitionMetrics, writePartitions map[int]*types.TaskListPartition) (map[int]*types.TaskListPartition, bool) {
	i.refreshGroups(metrics, writePartitions)
	partitionCount := i.getPartitionsPerGroup(writePartitions)

	partitionGoal := i.calculatePartitionGoal(metrics, partitionCount, len(writePartitions))

	// If there's a single partition we can't actually change anything, but we still want to run the above calculations
	// so that we record the correct sustain values. That way when a new partition is added we can immediately respond
	// with the correct number of pollers
	if len(writePartitions) == 1 {
		// If we resized to 1 partition, remove all assignments from that partition and clear all group status
		if len(writePartitions[0].IsolationGroups) != 0 {
			i.reset()
			return map[int]*types.TaskListPartition{
				0: {},
			}, true
		}
		return writePartitions, false
	}

	return i.applyGroupChanges(metrics, partitionCount, partitionGoal, writePartitions)
}

func (i *isolationBalancer) reset() {
	maps.Clear(i.groupState)
}

func (i *isolationBalancer) refreshGroups(metrics *aggregatePartitionMetrics, writePartitions map[int]*types.TaskListPartition) {
	polledOrAssigned := make(map[string]bool)
	// Track it for resizing/removal
	for _, p := range writePartitions {
		for _, group := range p.IsolationGroups {
			polledOrAssigned[group] = true
			if _, ok := i.groupState[group]; !ok {
				i.groupState[group] = i.createIsolationGroupState(true)
			}
		}
	}
	// Track it as we might assign it if they maintain pollers
	for group := range metrics.hasPollersByIsolationGroup {
		polledOrAssigned[group] = true
		if _, ok := i.groupState[group]; !ok {
			i.groupState[group] = i.createIsolationGroupState(false)
		}
	}
	for group := range i.groupState {
		// No pollers and no write partitions means we don't need to track it
		if !polledOrAssigned[group] {
			delete(i.groupState, group)
			continue
		}
	}
}

func (i *isolationBalancer) createIsolationGroupState(assignToPartitions bool) *isolationGroupState {
	return &isolationGroupState{
		underLoad:             clock.NewSustain(i.timeSource, i.config.IsolationGroupDownscaleSustainedDuration),
		overLoad:              clock.NewSustain(i.timeSource, i.config.IsolationGroupUpscaleSustainedDuration),
		noPollers:             clock.NewSustain(i.timeSource, i.config.IsolationGroupNoPollersSustainedDuration),
		hasPollers:            clock.NewSustain(i.timeSource, i.config.IsolationGroupHasPollersSustainedDuration),
		canAssignToPartitions: assignToPartitions,
	}
}

func (i *isolationBalancer) getPartitionsPerGroup(writePartitions map[int]*types.TaskListPartition) map[string]int {
	partitionCount := make(map[string]int)
	for _, p := range writePartitions {
		for _, group := range p.IsolationGroups {
			partitionCount[group]++
		}
	}
	return partitionCount
}

func (i *isolationBalancer) calculatePartitionGoal(aggregateMetrics *aggregatePartitionMetrics, partitionCountByGroup map[string]int, writePartitionCount int) map[string]int {
	partitionGoal := make(map[string]int)
	assignableGroups := 0
	// Identify groups that have switched to assignable and count them. This impacts the minimum partitions per group
	for group, state := range i.groupState {
		groupHasPollers := aggregateMetrics.hasPollersByIsolationGroup[group]
		sustainedPollers := state.hasPollers.CheckAndReset(groupHasPollers)
		sustainedNoPollers := state.noPollers.CheckAndReset(!groupHasPollers)
		if state.canAssignToPartitions {
			if sustainedNoPollers {
				i.scope.Tagged(metrics.IsolationGroupTag(group)).IncCounter(metrics.IsolationGroupStoppedPolling)
				state.canAssignToPartitions = false
			} else {
				assignableGroups++
			}
		} else if sustainedPollers {
			i.scope.Tagged(metrics.IsolationGroupTag(group)).IncCounter(metrics.IsolationGroupStartedPolling)
			state.canAssignToPartitions = true
			state.overLoad.Reset()
			state.underLoad.Reset()
			assignableGroups++
		}
	}

	minPartitions := i.getMinPartitionsPerGroup(writePartitionCount, assignableGroups)

	for group, state := range i.groupState {
		// Remove all assignments from inactive groups
		if !state.canAssignToPartitions {
			partitionGoal[group] = 0
			i.scope.Tagged(metrics.IsolationGroupTag(group)).UpdateGauge(metrics.IsolationGroupPartitionsGauge, 0)
			continue
		}
		currentPartitions := partitionCountByGroup[group]
		groupQPS := aggregateMetrics.qpsByIsolationGroup[group]

		upscaleRps := float64(i.config.PartitionUpscaleRPS()) / float64(i.config.IsolationGroupsPerPartition())
		downscaleFactor := i.config.PartitionDownscaleFactor()
		upscaleThreshold := float64(max(currentPartitions, 1)) * upscaleRps
		downscaleThreshold := float64(max(currentPartitions-1, 0)) * upscaleRps * downscaleFactor

		sustainedOverLoad := state.overLoad.Check(groupQPS > upscaleThreshold)
		sustainedUnderLoad := state.underLoad.Check(groupQPS < downscaleThreshold)

		idealPartitions := getNumberOfPartitions(groupQPS, upscaleRps)
		// partitions must be >= minPartitions and <= writePartitionCount
		targetPartitions := min(max(minPartitions, idealPartitions), writePartitionCount)

		// We may have a sustained overload but already be at the max number of partitions. Only reset the sustain
		// if we can actually make changes. This allows for instantly scaling up if the number of write partitions
		// changes
		// If currentPartitions == 0 then we just scaled up from not having pollers. Allow for more than the minimum
		if (sustainedOverLoad || sustainedUnderLoad || currentPartitions == 0) && targetPartitions != currentPartitions {
			if sustainedOverLoad {
				i.scope.Tagged(metrics.IsolationGroupTag(group)).IncCounter(metrics.IsolationGroupUpscale)
			} else if sustainedUnderLoad {
				i.scope.Tagged(metrics.IsolationGroupTag(group)).IncCounter(metrics.IsolationGroupDownscale)
			}
			state.overLoad.Reset()
			state.underLoad.Reset()
			partitionGoal[group] = targetPartitions
		} else {
			partitionGoal[group] = max(currentPartitions, minPartitions)
		}
		i.scope.Tagged(metrics.IsolationGroupTag(group)).UpdateGauge(metrics.IsolationGroupPartitionsGauge, float64(partitionGoal[group]))
	}

	return partitionGoal
}

func (i *isolationBalancer) applyGroupChanges(m *aggregatePartitionMetrics, partitionCount, partitionGoal map[string]int, partitions map[int]*types.TaskListPartition) (map[int]*types.TaskListPartition, bool) {
	groupSizePerPartition := make(map[string]float64)
	var toRemove []string
	var toAdd []string
	assignableGroups := 0
	for group, state := range i.groupState {
		if state.canAssignToPartitions {
			assignableGroups++
		}
		change := partitionGoal[group] - partitionCount[group]
		goalPartitions := partitionGoal[group]
		if goalPartitions > 0 {
			groupSizePerPartition[group] = m.qpsByIsolationGroup[group] / float64(goalPartitions)
		}
		for j := 0; j < change; j++ {
			toAdd = append(toAdd, group)
		}
		for j := change; j < 0; j++ {
			toRemove = append(toRemove, group)
		}
	}
	// partition id to set of groups
	partitionGroups := make(map[int]map[string]any)
	for id, partition := range partitions {
		set := make(map[string]any)
		for _, group := range partition.IsolationGroups {
			set[group] = true
		}
		partitionGroups[id] = set
	}
	removePartitions(toRemove, groupSizePerPartition, partitionGroups)
	addPartitions(toAdd, groupSizePerPartition, partitionGroups)
	minimumGroups := i.getMinGroupsPerPartition(assignableGroups)
	movedGroupBetweenPartitions := ensureMinimumGroupsPerPartition(minimumGroups, groupSizePerPartition, partitionGroups)
	result := make(map[int]*types.TaskListPartition)
	for id, groups := range partitionGroups {
		if len(groups) == 0 {
			result[id] = &types.TaskListPartition{}
		} else {
			asSlice := maps.Keys(groups)
			// Sort for the sake of stability
			slices.Sort(asSlice)
			result[id] = &types.TaskListPartition{IsolationGroups: asSlice}
		}
	}
	return result, movedGroupBetweenPartitions || len(toRemove) > 0 || len(toAdd) > 0
}

func (i *isolationBalancer) getMinPartitionsPerGroup(writePartitionCount, assignableGroups int) int {
	if assignableGroups == 0 {
		return 0
	}
	return int(math.Ceil(float64(writePartitionCount*i.getMinGroupsPerPartition(assignableGroups)) / float64(assignableGroups)))
}

func (i *isolationBalancer) getMinGroupsPerPartition(assignableGroups int) int {
	return min(i.config.IsolationGroupsPerPartition(), assignableGroups)
}

func removePartitions(toRemove []string, groupSizePerPartition map[string]float64, groupsByPartitionID map[int]map[string]any) {
	// Use a greedy approach to remove the specified groups (which may include duplicates) from the largest partition
	// Similarly to adding partitions we sort the input by size first
	slices.SortFunc(toRemove, byValueDescending(groupSizePerPartition))
	for _, group := range toRemove {
		partitionID, ok := findMaxWithGroup(group, groupSizePerPartition, groupsByPartitionID)
		if ok {
			delete(groupsByPartitionID[partitionID], group)
		}
	}
}

func addPartitions(toAdd []string, groupSizePerPartition map[string]float64, groupsByPartitionID map[int]map[string]any) {
	// Use greedy number partitioning to add the group to the smallest partition where it isn't already present. This
	// is an approximate solution. Sorting the input by size in a descending manner should result in a more even
	// distribution.
	slices.SortFunc(toAdd, byValueDescending(groupSizePerPartition))
	for _, group := range toAdd {
		partitionID, ok := findMinWithoutGroup(group, groupSizePerPartition, groupsByPartitionID)
		if ok {
			groupsByPartitionID[partitionID][group] = true
		}
	}
}

func ensureMinimumGroupsPerPartition(minGroups int, groupSizePerPartition map[string]float64, groupsByPartitionID map[int]map[string]any) bool {
	// Every partition needs to be assigned groups that have pollers, otherwise we're unable to process tasks on those
	// partitions.
	//
	// Sort the partitions by the number of isolation groups assigned to them, and then use a two-pointers approach
	// to iteratively take groups from partitions that have the most and move them to a partition that has the least.
	// calculatePartitionGoal enforces that there are at least "getMinPartitionsPerGroup" instances of each group,
	// which in turn ensures that every partition can be assigned "getMinGroupsPerPartition" by only moving groups
	// around.
	//
	// The specific groups that we move between partitions isn't particularly significant, since the size of each group
	// within a partition has an inherent upper-bound before we scale up. Ideally we keep partitions relatively close
	// in size, so we use a simple heuristic of minimizing the difference in total size between the two partitions that
	// we're considering.
	changed := false
	partitionIDsByGroupCount := maps.Keys(groupsByPartitionID)
	slices.SortFunc(partitionIDsByGroupCount, func(a, b int) int {
		aLen := len(groupsByPartitionID[a])
		bLen := len(groupsByPartitionID[b])
		if aLen < bLen {
			return -1
		} else if aLen == bLen {
			// Fall back to size ascending. Larger is later to prioritize moving groups from those partitions
			aSize := getPartitionSize(groupSizePerPartition, groupsByPartitionID[a])
			bSize := getPartitionSize(groupSizePerPartition, groupsByPartitionID[b])
			if aSize < bSize {
				return -1
			} else if aSize == bSize {
				return 0
			} else {
				return 1
			}
		} else {
			return 1
		}
	})
	lo, hi := 0, len(partitionIDsByGroupCount)-1
	for lo < hi {
		loPartition := groupsByPartitionID[partitionIDsByGroupCount[lo]]
		hiPartition := groupsByPartitionID[partitionIDsByGroupCount[hi]]
		if len(loPartition) >= minGroups {
			lo++
			continue
		}
		if len(hiPartition) <= minGroups {
			hi--
			continue
		}
		changed = true
		needed := minGroups - len(loPartition)
		available := len(hiPartition) - minGroups
		movePartitions(min(needed, available), hiPartition, loPartition, groupSizePerPartition)
	}
	return changed
}

func movePartitions(amount int, from, to map[string]any, groupSizePerPartition map[string]float64) {
	fromSize := getPartitionSize(groupSizePerPartition, from)
	toSize := getPartitionSize(groupSizePerPartition, to)
	// This is non-optimal when amount > 1 but it's good enough
	// Solving it ideally is number partitioning problem, exactly what we're trying to solve across all partitions
	for range amount {
		best := math.MaxFloat64
		bestGroup := ""
		for group := range from {
			if _, ok := to[group]; !ok {
				groupSize := groupSizePerPartition[group]
				diff := math.Abs(fromSize - toSize - (2 * groupSize))
				if diff < best {
					best = diff
					bestGroup = group
				}
			}
		}
		delete(from, bestGroup)
		fromSize -= groupSizePerPartition[bestGroup]
		toSize += groupSizePerPartition[bestGroup]
		to[bestGroup] = true
	}
}

func findMaxWithGroup(group string, groupSizePerPartition map[string]float64, groupsByPartitionID map[int]map[string]any) (int, bool) {
	maxSize := -math.MaxFloat64
	maxPartition := -1
	// iterate in order of partitionID to make decisions deterministic
	for partitionID := 0; partitionID < len(groupsByPartitionID); partitionID++ {
		set := groupsByPartitionID[partitionID]
		if _, ok := set[group]; ok {
			partitionSize := getPartitionSize(groupSizePerPartition, set)
			if maxSize < partitionSize {
				maxSize = partitionSize
				maxPartition = partitionID
			}
		}
	}
	return maxPartition, maxPartition != -1
}

func findMinWithoutGroup(group string, groupSizePerPartition map[string]float64, groupsByPartitionID map[int]map[string]any) (int, bool) {
	minSize := math.MaxFloat64
	minPartition := -1
	// iterate in order of partitionID to make decisions deterministic
	for partitionID := 0; partitionID < len(groupsByPartitionID); partitionID++ {
		set := groupsByPartitionID[partitionID]
		if _, ok := set[group]; !ok {
			partitionSize := getPartitionSize(groupSizePerPartition, set)
			if partitionSize < minSize {
				minSize = partitionSize
				minPartition = partitionID
			}
		}
	}
	return minPartition, minPartition != -1
}

func getPartitionSize(groupSizePerPartition map[string]float64, groupSet map[string]any) float64 {
	total := float64(0)
	for group := range groupSet {
		total += groupSizePerPartition[group]
	}
	return total
}

func byValueDescending[T comparable](m map[T]float64) func(a, b T) int {
	return func(a, b T) int {
		aVal := m[a]
		bVal := m[b]
		if aVal == bVal {
			return 0
		} else if aVal > bVal {
			return -1
		} else {
			return 1
		}
	}
}
