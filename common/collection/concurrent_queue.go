// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package collection

import (
	"errors"
	"sync"
)

type (
	concurrentQueueImpl[T any] struct {
		sync.RWMutex
		items []T
	}
)

// NewConcurrentQueue creates a new concurrent queue
func NewConcurrentQueue[T any]() Queue[T] {
	return &concurrentQueueImpl[T]{
		items: make([]T, 0, 1000),
	}
}

func (q *concurrentQueueImpl[T]) Peek() (T, error) {
	q.RLock()
	defer q.RUnlock()

	var item T
	if q.isEmptyLocked() {
		return item, errors.New("queue is empty")
	}
	return q.items[0], nil
}

func (q *concurrentQueueImpl[T]) Add(item T) {
	q.Lock()
	defer q.Unlock()

	q.items = append(q.items, item)
}

func (q *concurrentQueueImpl[T]) Remove() (T, error) {
	q.Lock()
	defer q.Unlock()
	var item T
	if q.isEmptyLocked() {
		return item, errors.New("queue is empty")
	}

	item = q.items[0]
	q.items = q.items[1:]

	return item, nil
}

func (q *concurrentQueueImpl[T]) IsEmpty() bool {
	q.RLock()
	defer q.RUnlock()

	return q.isEmptyLocked()
}

func (q *concurrentQueueImpl[T]) Len() int {
	q.RLock()
	defer q.RUnlock()

	return len(q.items)
}

func (q *concurrentQueueImpl[T]) isEmptyLocked() bool {
	return len(q.items) == 0
}
