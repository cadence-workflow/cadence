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

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/task"
)

type (
	VirtualSlice interface {
		GetState() VirtualSliceState
		IsEmpty() bool
		GetTasks(context.Context, int) ([]task.Task, error)
		InsertTask(task.Task) bool
		HasMoreTasks() bool
		UpdateAndGetState() VirtualSliceState
		GetPendingTaskCount() int
		Clear()
		PendingTaskStats() PendingTaskStats
		ResetProgress(key persistence.HistoryTaskKey)

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
		logger             log.Logger

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
	logger log.Logger,
) VirtualSlice {
	return &virtualSliceImpl{
		state:              state,
		taskInitializer:    taskInitializer,
		queueReader:        queueReader,
		pendingTaskTracker: pendingTaskTracker,
		logger:             logger,
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

func (s *virtualSliceImpl) IsEmpty() bool {
	return s.state.IsEmpty() && !s.HasMoreTasks()
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

func (s *virtualSliceImpl) InsertTask(task task.Task) bool {
	if s.state.Contains(task) {
		s.pendingTaskTracker.AddTask(task)
		return true
	}

	return false
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
			// NOTE: we must return the tasks here to let them either be submitted to scheduler or rescheduler
			// because they are already added to pending task tracker. Otherwise, they will become zombie tasks,
			// and won't be processed until shard restart.
			// The number of tasks returned here doesn't need to be the same as the page size even if there is still more tasks to read.
			// HasMoreTasks() method will still return true in this case.
			if len(tasks) > 0 {
				return tasks, nil
			}
			return nil, err
		}

		for _, t := range resp.Tasks {
			task := s.taskInitializer(t)
			tasks = append(tasks, task)
			s.pendingTaskTracker.AddTask(task)
		}

		// The persistence layer may return non-empty next page token even if there are no more tasks to read
		// We compare the next task key with the exclusive max task key to determine if there are more tasks to read instead
		if resp.Progress.NextTaskKey.Compare(s.progress[0].ExclusiveMaxTaskKey) < 0 {
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
	prunedCount := s.pendingTaskTracker.PruneAckedTasks()
	nextTaskKey := s.state.Range.ExclusiveMaxTaskKey
	if len(s.progress) > 0 {
		nextTaskKey = s.progress[0].NextTaskKey
	}
	s.logger.Debug("pruned acked tasks", tag.Counter(prunedCount), tag.Dynamic("inclusiveMinTaskKey", s.state.Range.InclusiveMinTaskKey), tag.Dynamic("exclusiveMaxTaskKey", s.state.Range.ExclusiveMaxTaskKey), tag.Dynamic("nextTaskKey", nextTaskKey))
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

// this function is used when we are not sure if our in-memory state after the given key is correct,
// we want to cancel all the tasks after the given key and reset the progress to the given key,
// so that in the next poll, we will read the tasks from the DB starting from the given key.
func (s *virtualSliceImpl) ResetProgress(key persistence.HistoryTaskKey) {

	// the given key is after the current slice, no need to reset
	if key.Compare(s.state.Range.ExclusiveMaxTaskKey) >= 0 {
		return
	}

	taskMap := s.pendingTaskTracker.GetTasks()
	for _, task := range taskMap {
		if task.GetTaskKey().Compare(key) >= 0 {
			task.Cancel()
		}
	}

	if len(s.progress) == 0 {
		s.progress = []*GetTaskProgress{
			{
				Range: Range{
					InclusiveMinTaskKey: key,
					ExclusiveMaxTaskKey: s.state.Range.ExclusiveMaxTaskKey,
				},
				NextPageToken: nil,
				NextTaskKey:   key,
			},
		}
		return
	}

	for i, progress := range s.progress {
		// progress contains sorted non-overlapping ranges. If we found a range that contains the given key, we can reset
		// this range's progress to the given key and merge the remaining ranges into it.
		if progress.ExclusiveMaxTaskKey.Compare(key) > 0 {
			maxTaskKey := s.progress[len(s.progress)-1].Range.ExclusiveMaxTaskKey
			s.progress[i] = &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.MinHistoryTaskKey(key, progress.InclusiveMinTaskKey),
					ExclusiveMaxTaskKey: maxTaskKey,
				},
				NextPageToken: nil,
				NextTaskKey:   persistence.MinHistoryTaskKey(key, progress.NextTaskKey),
			}

			s.state.Range.InclusiveMinTaskKey = persistence.MinHistoryTaskKey(key, s.state.Range.InclusiveMinTaskKey)
			s.progress = s.progress[:i+1]
			break
		}
	}
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
		logger:             s.logger,
	}

	rightSlice := &virtualSliceImpl{
		state:              rightState,
		taskInitializer:    s.taskInitializer,
		queueReader:        s.queueReader,
		pendingTaskTracker: rightTracker,
		progress:           rightProgress,
		logger:             s.logger,
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
		logger:             s.logger,
	}
	failSlice := &virtualSliceImpl{
		state:              failState,
		taskInitializer:    s.taskInitializer,
		queueReader:        s.queueReader,
		pendingTaskTracker: failTracker,
		progress:           failProgress,
		logger:             s.logger,
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
		logger:             left.logger,
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
		logger:             this.logger,
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
