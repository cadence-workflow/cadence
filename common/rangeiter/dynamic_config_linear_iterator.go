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

package rangeiter

import (
	"fmt"
	"sync"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

// DynamicConfigLinearIterator is a LinearIterator that can be reconfigured dynamically.
// It is safe for concurrent use. Every operation checks the values of min, max, and stepCount in dynamic config
// and refreshes the iterator if they have changed.
type DynamicConfigLinearIterator[T Integer] struct {
	mx     sync.Mutex
	logger log.Logger

	iter    *LinearIterator[T]
	curSpec *linearIteratorSpec[T]

	minFn       func() T
	maxFn       func() T
	stepCountFn func() int
}

// NewDynamicConfigLinearIterator creates a new DynamicConfigLinearIterator.
func NewDynamicConfigLinearIterator[T Integer](min, max func() T, stepCount func() int, logger log.Logger) *DynamicConfigLinearIterator[T] {
	iter := &DynamicConfigLinearIterator[T]{
		minFn:       min,
		maxFn:       max,
		stepCountFn: stepCount,
		logger:      logger,
	}
	iter.curSpec = iter.newLinearIteratorSpec()
	iter.iter = iter.curSpec.newIterator()

	iter.logger.Info("Dynamic config linear iterator initialized",
		tag.DynamicConfigLinearIteratorSpec(iter.curSpec),
	)
	return iter
}

// refreshIterator checks if the spec has changed and refreshes the iterator if it has.
func (d *DynamicConfigLinearIterator[T]) refreshIterator() {
	newSpec := d.newLinearIteratorSpec()
	if newSpec.equal(d.curSpec) {
		return
	}

	d.logger.Info("Dynamic config linear iterator spec has changed, refreshing iterator, applying new spec",
		tag.DynamicConfigLinearIteratorSpec(newSpec),
	)

	d.curSpec = newSpec
	d.iter = newSpec.newIterator()
}

// newLinearIteratorSpec creates a new linearIteratorSpec from the current dynamic config.
func (d *DynamicConfigLinearIterator[T]) newLinearIteratorSpec() *linearIteratorSpec[T] {
	return &linearIteratorSpec[T]{
		min:       d.minFn(),
		max:       d.maxFn(),
		stepCount: d.stepCountFn(),
	}
}

// Next returns the next value in the iterator.
func (d *DynamicConfigLinearIterator[T]) Next() T {
	d.mx.Lock()
	defer d.mx.Unlock()

	d.refreshIterator()
	return d.iter.Next()
}

// Previous returns the previous value in the iterator.
func (d *DynamicConfigLinearIterator[T]) Previous() T {
	d.mx.Lock()
	defer d.mx.Unlock()

	d.refreshIterator()
	return d.iter.Previous()
}

// Value returns the current value in the iterator.
func (d *DynamicConfigLinearIterator[T]) Value() T {
	d.mx.Lock()
	defer d.mx.Unlock()

	d.refreshIterator()
	return d.iter.Value()
}

// Reset resets the iterator to the beginning.
func (d *DynamicConfigLinearIterator[T]) Reset() {
	d.mx.Lock()
	defer d.mx.Unlock()

	d.refreshIterator()
	d.iter.Reset()
}

// linearIteratorSpec is a specification for a LinearIterator
type linearIteratorSpec[T Integer] struct {
	min, max  T
	stepCount int
}

// String returns a string representation of the spec.
func (s *linearIteratorSpec[T]) String() string {
	return fmt.Sprintf("min: %v, max: %v, stepCount: %v", s.min, s.max, s.stepCount)
}

// newIterator creates a new LinearIterator from the spec.
func (s *linearIteratorSpec[T]) newIterator() *LinearIterator[T] {
	return NewLinearIterator(s.min, s.max, s.stepCount)
}

// equal returns true if the spec is equal to another spec.
func (s *linearIteratorSpec[T]) equal(b *linearIteratorSpec[T]) bool {
	return s.min == b.min && s.max == b.max && s.stepCount == b.stepCount
}
