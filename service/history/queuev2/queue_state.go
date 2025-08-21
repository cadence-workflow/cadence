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

package queuev2

import "github.com/uber/cadence/common/persistence"

type QueueState struct {
	VirtualQueueStates    map[int64][]VirtualSliceState
	ExclusiveMaxReadLevel persistence.HistoryTaskKey
}

type VirtualSliceState struct {
	Range     Range
	Predicate Predicate
}

func (s *VirtualSliceState) IsEmpty() bool {
	return s.Range.IsEmpty() || s.Predicate.IsEmpty()
}

func (s *VirtualSliceState) Contains(task persistence.Task) bool {
	return s.Range.Contains(task.GetTaskKey()) && s.Predicate.Check(task)
}

func (s *VirtualSliceState) TrySplitByTaskKey(taskKey persistence.HistoryTaskKey) (VirtualSliceState, VirtualSliceState, bool) {
	if !s.Range.CanSplitByTaskKey(taskKey) {
		return VirtualSliceState{}, VirtualSliceState{}, false
	}

	return VirtualSliceState{
			Range:     Range{InclusiveMinTaskKey: s.Range.InclusiveMinTaskKey, ExclusiveMaxTaskKey: taskKey},
			Predicate: s.Predicate,
		}, VirtualSliceState{
			Range:     Range{InclusiveMinTaskKey: taskKey, ExclusiveMaxTaskKey: s.Range.ExclusiveMaxTaskKey},
			Predicate: s.Predicate,
		}, true
}

func (s *VirtualSliceState) TrySplitByPredicate(predicate Predicate) (VirtualSliceState, VirtualSliceState, bool) {
	if predicate.Equals(&universalPredicate{}) || predicate.Equals(&emptyPredicate{}) || predicate.Equals(s.Predicate) {
		return VirtualSliceState{}, VirtualSliceState{}, false
	}
	return VirtualSliceState{
			Range:     s.Range,
			Predicate: And(s.Predicate, predicate),
		}, VirtualSliceState{
			Range:     s.Range,
			Predicate: And(s.Predicate, Not(predicate)),
		}, true
}
