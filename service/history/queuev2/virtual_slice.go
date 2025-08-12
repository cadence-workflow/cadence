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

//go:generate mockgen -package $GOPACKAGE -destination virtual_slice_mock.go github.com/uber/cadence/service/history/queuev2 VirtualSlice
package queuev2

import (
	"context"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/task"
)

type (
	VirtualSlice interface {
		GetState() VirtualSliceState
		GetTasks(context.Context, int) ([]task.Task, error)
		HasMoreTasks() bool
		UpdateAndGetState() VirtualSliceState
		GetPendingTaskCount() int
		Clear()
		PendingTaskStats() PendingTaskStats

		TrySplitByTaskKey(persistence.HistoryTaskKey) (VirtualSlice, VirtualSlice, bool)
		TrySplitByPredicate(Predicate) (VirtualSlice, VirtualSlice, bool)
		TryMergeWithVirtualSlice(VirtualSlice) ([]VirtualSlice, bool)
	}

	PendingTaskStats struct {
		PendingTaskCountPerDomain map[string]int
	}

	virtualSliceImpl struct {
		state              VirtualSliceState
		taskInitializer    task.Initializer
		queueReader        QueueReader
		pendingTaskTracker PendingTaskTracker

		// progress tracks the read progress of the slice, sorted by the inclusive min task key of the range, ranges are not overlapping
		// For a virtual slice, the progress is a task key pointing to the next task to read and the next page token
		// In most cases, there is only one GetTaskProgress item in the progress slice.
		// However, when 2 virtual slices are merged, the progress slice may contain 2 items, because even though the range of the slices have overlap,
		// their GetTaskProgress cannot be merged, because the next page tokens are coupled with the range of the original slices.
		progress []*GetTaskProgress
	}
)

func NewVirtualSlice(
	state VirtualSliceState,
	taskInitializer task.Initializer,
	queueReader QueueReader,
	pendingTaskTracker PendingTaskTracker,
) VirtualSlice {
	return &virtualSliceImpl{
		state:              state,
		taskInitializer:    taskInitializer,
		queueReader:        queueReader,
		pendingTaskTracker: pendingTaskTracker,
		progress: []*GetTaskProgress{
			{
				Range:         state.Range,
				NextPageToken: nil,
				NextTaskKey:   state.Range.InclusiveMinTaskKey,
			},
		},
	}
}

func (s *virtualSliceImpl) GetState() VirtualSliceState {
	return s.state
}

func (s *virtualSliceImpl) GetPendingTaskCount() int {
	return s.pendingTaskTracker.GetPendingTaskCount()
}

func (s *virtualSliceImpl) Clear() {
	s.UpdateAndGetState()
	s.pendingTaskTracker.Clear()
	s.progress = []*GetTaskProgress{
		{
			Range:         s.state.Range,
			NextPageToken: nil,
			NextTaskKey:   s.state.Range.InclusiveMinTaskKey,
		},
	}
}

func (s *virtualSliceImpl) GetTasks(ctx context.Context, pageSize int) ([]task.Task, error) {
	if len(s.progress) == 0 {
		return nil, nil
	}

	tasks := make([]task.Task, 0, pageSize)
	for len(tasks) < pageSize && len(s.progress) > 0 {
		resp, err := s.queueReader.GetTask(ctx, &GetTaskRequest{
			Progress:  s.progress[0],
			Predicate: s.state.Predicate,
			PageSize:  pageSize - len(tasks),
		})
		if err != nil {
			return nil, err
		}

		for _, t := range resp.Tasks {
			task := s.taskInitializer(t)
			tasks = append(tasks, task)
			s.pendingTaskTracker.AddTask(task)
		}

		if len(resp.Progress.NextPageToken) != 0 {
			s.progress[0] = resp.Progress
		} else {
			s.progress = s.progress[1:]
		}
	}

	return tasks, nil
}

func (s *virtualSliceImpl) HasMoreTasks() bool {
	return len(s.progress) > 0
}

func (s *virtualSliceImpl) PendingTaskStats() PendingTaskStats {
	return PendingTaskStats{
		PendingTaskCountPerDomain: s.pendingTaskTracker.GetPerDomainPendingTaskCount(),
	}
}

func (s *virtualSliceImpl) UpdateAndGetState() VirtualSliceState {
	s.pendingTaskTracker.PruneAckedTasks()
	minPendingTaskKey, ok := s.pendingTaskTracker.GetMinimumTaskKey()
	if !ok {
		if len(s.progress) > 0 { // no pending tasks, and there are more tasks to read
			s.state.Range.InclusiveMinTaskKey = s.progress[0].NextTaskKey
		} else { // no pending tasks, and no more tasks to read
			s.state.Range.InclusiveMinTaskKey = s.state.Range.ExclusiveMaxTaskKey
		}
	} else {
		if len(s.progress) > 0 { // there are pending tasks, and there are more tasks to read
			s.state.Range.InclusiveMinTaskKey = persistence.MinHistoryTaskKey(minPendingTaskKey, s.progress[0].NextTaskKey)
		} else { // there are pending tasks, and no more tasks to read
			s.state.Range.InclusiveMinTaskKey = minPendingTaskKey
		}
	}
	return s.state
}

func (s *virtualSliceImpl) TrySplitByTaskKey(taskKey persistence.HistoryTaskKey) (VirtualSlice, VirtualSlice, bool) {
	leftState, rightState, ok := s.state.TrySplitByTaskKey(taskKey)
	if !ok {
		return nil, nil, false
	}

	leftTracker := NewPendingTaskTracker()
	rightTracker := NewPendingTaskTracker()

	taskMap := s.pendingTaskTracker.GetTasks()
	for taskKey, task := range taskMap {
		if leftState.Range.Contains(taskKey) {
			leftTracker.AddTask(task)
		} else {
			rightTracker.AddTask(task)
		}
	}

	leftProgress := []*GetTaskProgress{}
	rightProgress := []*GetTaskProgress{}

	for _, progress := range s.progress {
		if leftState.Range.ContainsRange(progress.Range) {
			leftProgress = append(leftProgress, progress)
			continue
		}
		if rightState.Range.ContainsRange(progress.Range) {
			rightProgress = append(rightProgress, progress)
			continue
		}

		if leftState.Range.Contains(progress.NextTaskKey) {
			leftProgress = append(leftProgress, &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: progress.NextTaskKey,
					ExclusiveMaxTaskKey: leftState.Range.ExclusiveMaxTaskKey,
				},
				NextPageToken: nil,
				NextTaskKey:   progress.NextTaskKey,
			})
			rightProgress = append(rightProgress, &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: rightState.Range.InclusiveMinTaskKey,
					ExclusiveMaxTaskKey: progress.Range.ExclusiveMaxTaskKey,
				},
				NextPageToken: nil,
				NextTaskKey:   rightState.Range.InclusiveMinTaskKey,
			})
		} else {
			rightProgress = append(rightProgress, &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: progress.NextTaskKey,
					ExclusiveMaxTaskKey: progress.Range.ExclusiveMaxTaskKey,
				},
				NextPageToken: nil,
				NextTaskKey:   progress.NextTaskKey,
			})
		}
	}

	leftSlice := &virtualSliceImpl{
		state:              leftState,
		taskInitializer:    s.taskInitializer,
		queueReader:        s.queueReader,
		pendingTaskTracker: leftTracker,
		progress:           leftProgress,
	}

	rightSlice := &virtualSliceImpl{
		state:              rightState,
		taskInitializer:    s.taskInitializer,
		queueReader:        s.queueReader,
		pendingTaskTracker: rightTracker,
		progress:           rightProgress,
	}

	return leftSlice, rightSlice, true
}

func (s *virtualSliceImpl) TrySplitByPredicate(predicate Predicate) (VirtualSlice, VirtualSlice, bool) {
	passState, failState, ok := s.state.TrySplitByPredicate(predicate)
	if !ok {
		return nil, nil, false
	}
	passTracker := NewPendingTaskTracker()
	failTracker := NewPendingTaskTracker()

	taskMap := s.pendingTaskTracker.GetTasks()
	for _, task := range taskMap {
		if passState.Predicate.Check(task) {
			passTracker.AddTask(task)
		} else {
			failTracker.AddTask(task)
		}
	}

	passProgress := make([]*GetTaskProgress, len(s.progress))
	failProgress := make([]*GetTaskProgress, len(s.progress))
	copy(passProgress, s.progress)
	copy(failProgress, s.progress)
	passSlice := &virtualSliceImpl{
		state:              passState,
		taskInitializer:    s.taskInitializer,
		queueReader:        s.queueReader,
		pendingTaskTracker: passTracker,
		progress:           passProgress,
	}
	failSlice := &virtualSliceImpl{
		state:              failState,
		taskInitializer:    s.taskInitializer,
		queueReader:        s.queueReader,
		pendingTaskTracker: failTracker,
		progress:           failProgress,
	}
	return passSlice, failSlice, true
}

func (s *virtualSliceImpl) TryMergeWithVirtualSlice(other VirtualSlice) ([]VirtualSlice, bool) {
	otherImpl, ok := other.(*virtualSliceImpl)
	if !ok {
		return nil, false
	}

	if s == other || !s.state.Range.CanMerge(other.GetState().Range) {
		return nil, false
	}

	if s.state.Predicate.Equals(other.GetState().Predicate) {
		return []VirtualSlice{mergeVirtualSlicesByRange(s, otherImpl)}, true
	}
	return mergeVirtualSlicesWithDifferentPredicate(s, otherImpl)
}

func mergeVirtualSlicesByRange(left, right *virtualSliceImpl) VirtualSlice {
	if left.state.Range.InclusiveMinTaskKey.Compare(right.state.Range.InclusiveMinTaskKey) > 0 {
		left, right = right, left
	}
	mergedState := VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: left.state.Range.InclusiveMinTaskKey,
			ExclusiveMaxTaskKey: persistence.MaxHistoryTaskKey(left.state.Range.ExclusiveMaxTaskKey, right.state.Range.ExclusiveMaxTaskKey),
		},
		Predicate: left.state.Predicate, // left and right have the same predicate
	}
	pendingTaskTracker := left.pendingTaskTracker
	taskMap := right.pendingTaskTracker.GetTasks()
	for _, task := range taskMap {
		pendingTaskTracker.AddTask(task)
	}
	mergedProgress := mergeGetTaskProgressWithSamePredicate(left.progress, right.progress)

	return &virtualSliceImpl{
		state:              mergedState,
		taskInitializer:    left.taskInitializer,
		queueReader:        left.queueReader,
		pendingTaskTracker: pendingTaskTracker,
		progress:           mergedProgress,
	}
}

func mergeGetTaskProgressWithSamePredicate(left, right []*GetTaskProgress) []*GetTaskProgress {
	mergedProgress := []*GetTaskProgress{}
	leftIndex := 0
	rightIndex := 0
	for leftIndex < len(left) && rightIndex < len(right) {
		if left[leftIndex].Range.InclusiveMinTaskKey.Compare(right[rightIndex].Range.InclusiveMinTaskKey) <= 0 {
			mergedProgress = appendOrMergeProgressWithSamePredicate(mergedProgress, left[leftIndex])
			leftIndex++
		} else {
			mergedProgress = appendOrMergeProgressWithSamePredicate(mergedProgress, right[rightIndex])
			rightIndex++
		}
	}
	for leftIndex < len(left) {
		mergedProgress = appendOrMergeProgressWithSamePredicate(mergedProgress, left[leftIndex])
		leftIndex++
	}
	for rightIndex < len(right) {
		mergedProgress = appendOrMergeProgressWithSamePredicate(mergedProgress, right[rightIndex])
		rightIndex++
	}
	return mergedProgress
}

func appendOrMergeProgressWithSamePredicate(mergedProgress []*GetTaskProgress, progress *GetTaskProgress) []*GetTaskProgress {
	if len(mergedProgress) == 0 {
		return append(mergedProgress, progress)
	}

	lastProgress := mergedProgress[len(mergedProgress)-1]
	mergedProgress = mergedProgress[:len(mergedProgress)-1] // remove the last progress
	return append(mergedProgress, mergeProgressWithSamePredicate(lastProgress, progress)...)
}

// mergeProgress merges two progress with the same predicate
// Assuming the inclusive key, next key, max key of the 2 progress are [a, b, c] and [x, y, z] where a <= b <= c and x <= y <= z,
// also assuming that a <= x, otherwise we can swap the left and right progress
// There are 10 different cases regarding the order of a, b, c, x, y, z, and here are the cases and merged results:
// [a, b, c, x, y, z] -> [b, b, c] and [y, y, z]
// [a, b, x, c, y, z] -> [b, b, x] and [y, y, z]
// [a, b, x, y, c, z] -> [b, b, x] and [y, y, z]
// [a, b, x, y, z, c] -> [b, b, x] and [y, y, c]
// [a, x, b, c, y ,z] -> [y, y, z]
// [a, x, b, y, c, z] -> [y, y, z]
// [a, x, b, y, z, c] -> [y, y, c]
// [a, x, y, b, c, z] -> [b, b, z]
// [a, x, y, b, z, c] -> [b, b, c]
// [a, x, y, z, b, c] -> [b, b, c]
// we only need to consider the range that hasn't been read yet, the merged result can be represented as
// [b, b, min(c, x)], [max(b, y), max(b, y), max(c, z)], and if b >= x, [b, b, min(c, x)] will be an empty progress so it's omitted
func mergeProgressWithSamePredicate(left, right *GetTaskProgress) []*GetTaskProgress {
	if left.Range.InclusiveMinTaskKey.Compare(right.Range.InclusiveMinTaskKey) > 0 {
		left, right = right, left
	}
	if left.NextTaskKey.Compare(right.InclusiveMinTaskKey) < 0 {
		return []*GetTaskProgress{
			{
				Range: Range{
					InclusiveMinTaskKey: left.NextTaskKey,
					ExclusiveMaxTaskKey: persistence.MinHistoryTaskKey(left.ExclusiveMaxTaskKey, right.InclusiveMinTaskKey),
				},
				NextPageToken: nil,
				NextTaskKey:   left.NextTaskKey,
			},
			{
				Range: Range{
					InclusiveMinTaskKey: right.NextTaskKey,
					ExclusiveMaxTaskKey: persistence.MaxHistoryTaskKey(left.ExclusiveMaxTaskKey, right.ExclusiveMaxTaskKey),
				},
				NextPageToken: nil,
				NextTaskKey:   persistence.MaxHistoryTaskKey(left.NextTaskKey, right.NextTaskKey),
			},
		}
	}
	return []*GetTaskProgress{
		{
			Range: Range{
				InclusiveMinTaskKey: persistence.MaxHistoryTaskKey(left.NextTaskKey, right.NextTaskKey),
				ExclusiveMaxTaskKey: persistence.MaxHistoryTaskKey(left.ExclusiveMaxTaskKey, right.ExclusiveMaxTaskKey),
			},
			NextPageToken: nil,
			NextTaskKey:   persistence.MaxHistoryTaskKey(left.NextTaskKey, right.NextTaskKey),
		},
	}
}

func mergeGetTaskProgressWithDifferentPredicate(left, right []*GetTaskProgress) []*GetTaskProgress {
	mergedProgress := []*GetTaskProgress{}
	leftIndex := 0
	rightIndex := 0
	for leftIndex < len(left) && rightIndex < len(right) {
		if left[leftIndex].NextTaskKey.Compare(right[rightIndex].NextTaskKey) <= 0 {
			mergedProgress = appendOrMergeProgressWithDifferentPredicate(mergedProgress, left[leftIndex])
			leftIndex++
		} else {
			mergedProgress = appendOrMergeProgressWithDifferentPredicate(mergedProgress, right[rightIndex])
			rightIndex++
		}
	}
	for leftIndex < len(left) {
		mergedProgress = appendOrMergeProgressWithDifferentPredicate(mergedProgress, left[leftIndex])
		leftIndex++
	}
	for rightIndex < len(right) {
		mergedProgress = appendOrMergeProgressWithDifferentPredicate(mergedProgress, right[rightIndex])
		rightIndex++
	}
	return mergedProgress
}

func appendOrMergeProgressWithDifferentPredicate(mergedProgress []*GetTaskProgress, progress *GetTaskProgress) []*GetTaskProgress {
	if len(mergedProgress) == 0 {
		return append(mergedProgress, progress)
	}

	lastProgress := mergedProgress[len(mergedProgress)-1]
	mergedProgress = mergedProgress[:len(mergedProgress)-1] // remove the last progress
	return append(mergedProgress, mergeProgressWithDifferentPredicate(lastProgress, progress)...)
}

// mergeProgress merges two progress with different predicates
// Assuming the inclusive key, next key, max key of the 2 progress are [a, b, c] and [x, y, z] where a <= b <= c and x <= y <= z,
// also assuming that b <= y, otherwise we can swap the left and right progress
// There are 7 different cases regarding the order of a, b, c, x, y, z, and here are the cases and merged results:
// [a, b, c, x, y, z] -> [b, b, c] and [y, y, z], (c < y)
// [a, b, x, c, y, z] -> [b, b, c] and [y, y, z], (c < y)
// [a, b, x, y, c, z] -> [b, b, z]
// [a, b, x, y, z, c] -> [b, b, c]
// [a, x, b, c, y ,z] -> [b, b, c] and [y, y, z], (c < y)
// [a, x, b, y, c, z] -> [b, b, z]
// [a, x, b, y, z, c] -> [b, b, c]
// the idea is to get the union of [next task key, exclusive max task key) of the 2 progress,
// and then set inclusive min task key to next task key, next page token to nil
func mergeProgressWithDifferentPredicate(left, right *GetTaskProgress) []*GetTaskProgress {
	if left.NextTaskKey.Compare(right.NextTaskKey) > 0 {
		left, right = right, left
	}

	if left.Range.ExclusiveMaxTaskKey.Compare(right.NextTaskKey) < 0 {
		return []*GetTaskProgress{
			{
				Range: Range{
					InclusiveMinTaskKey: left.NextTaskKey,
					ExclusiveMaxTaskKey: left.ExclusiveMaxTaskKey,
				},
				NextPageToken: nil,
				NextTaskKey:   left.NextTaskKey,
			},
			{
				Range: Range{
					InclusiveMinTaskKey: right.NextTaskKey,
					ExclusiveMaxTaskKey: right.ExclusiveMaxTaskKey,
				},
				NextPageToken: nil,
				NextTaskKey:   right.NextTaskKey,
			},
		}
	}
	return []*GetTaskProgress{
		{
			Range: Range{
				InclusiveMinTaskKey: left.NextTaskKey,
				ExclusiveMaxTaskKey: persistence.MaxHistoryTaskKey(left.ExclusiveMaxTaskKey, right.ExclusiveMaxTaskKey),
			},
			NextPageToken: nil,
			NextTaskKey:   left.NextTaskKey,
		},
	}
}

func mergeVirtualSlicesByPredicate(this, that *virtualSliceImpl) VirtualSlice {
	mergedState := VirtualSliceState{
		Range:     this.state.Range, // this and that have the same range
		Predicate: Or(this.state.Predicate, that.state.Predicate),
	}
	pendingTaskTracker := this.pendingTaskTracker
	taskMap := that.pendingTaskTracker.GetTasks()
	for _, task := range taskMap {
		pendingTaskTracker.AddTask(task)
	}
	mergedProgress := mergeGetTaskProgressWithDifferentPredicate(this.progress, that.progress)

	return &virtualSliceImpl{
		state:              mergedState,
		taskInitializer:    this.taskInitializer,
		queueReader:        this.queueReader,
		pendingTaskTracker: pendingTaskTracker,
		progress:           mergedProgress,
	}
}

// merge slices with different predicates, the general idea is to find the overlap range of the 2 slices,
// combine the predicates of the overlap range and leave the rest as is
func mergeVirtualSlicesWithDifferentPredicate(this, that *virtualSliceImpl) ([]VirtualSlice, bool) {
	compareInclusiveMin := this.state.Range.InclusiveMinTaskKey.Compare(that.state.Range.InclusiveMinTaskKey)
	if compareInclusiveMin > 0 {
		this, that = that, this
	} else if compareInclusiveMin == 0 {
		if this.state.Range.ExclusiveMaxTaskKey.Compare(that.state.Range.ExclusiveMaxTaskKey) > 0 {
			this, that = that, this
		}
	}
	// Use a, b to to represent the inclusive min task key and exclusive max task key of `this`
	// Use x, y to to represent the inclusive min task key and exclusive max task key of `that`
	// At this point, we know that a <= x, (in actual world, we also know that x <= b because we know that the 2 slices can be merged, but it doesn't affect the logic), so there are 5 cases to consider:
	// 1. {a, b, x, y} (x >= b, a < x) -> don't merge
	// 2. {a, x, b, y} (a < x < b < y) -> [a, x) and [x, b) and [b, y)
	// 3. {a, x, b, y} (a < x < b == y) -> [a, x) and [x, b)
	// 4. {a, x, y, b} (a < x < y < b) -> [a, x) and [x, y) and [y, b)
	// 5. {x, a, b, y} (x == a < b < y) -> [x, b) and [b, y)
	// 6. {x, a, y, b} (x == a < y == b) -> [x, b)
	if compareInclusiveMin == 0 {
		thatLeft, thatRight, ok := that.TrySplitByTaskKey(this.state.Range.ExclusiveMaxTaskKey)
		if !ok {
			// Case 6
			return []VirtualSlice{mergeVirtualSlicesByPredicate(this, that)}, true
		}
		// Case 5
		return []VirtualSlice{mergeVirtualSlicesByPredicate(this, thatLeft.(*virtualSliceImpl)), thatRight}, true
	}
	thisLeft, thisRight, ok := this.TrySplitByTaskKey(that.state.Range.InclusiveMinTaskKey)
	if !ok {
		// Case 1
		return nil, false
	}
	mergedVirtualSlices := make([]VirtualSlice, 0, 3)
	mergedVirtualSlices = append(mergedVirtualSlices, thisLeft)
	thatLeft, thatRight, ok := that.TrySplitByTaskKey(this.state.Range.ExclusiveMaxTaskKey)
	if !ok {
		thisRightLeft, thisRightRight, ok := thisRight.TrySplitByTaskKey(that.state.Range.ExclusiveMaxTaskKey)
		if !ok {
			// Case 3: combine predicates of thisRight and that
			mergedVirtualSlices = append(mergedVirtualSlices, mergeVirtualSlicesByPredicate(thisRight.(*virtualSliceImpl), that))
			return mergedVirtualSlices, true
		}
		// Case 4: combine predicates of thisRightLeft and that
		mergedVirtualSlices = append(mergedVirtualSlices, mergeVirtualSlicesByPredicate(thisRightLeft.(*virtualSliceImpl), that))
		mergedVirtualSlices = append(mergedVirtualSlices, thisRightRight)
		return mergedVirtualSlices, true
	}
	// Case 2: combine predicates of thisRight and thatLeft
	mergedVirtualSlices = append(mergedVirtualSlices, mergeVirtualSlicesByPredicate(thisRight.(*virtualSliceImpl), thatLeft.(*virtualSliceImpl)))
	mergedVirtualSlices = append(mergedVirtualSlices, thatRight)
	return mergedVirtualSlices, true
}
