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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/log/testlogger"
)

func TestDynamicConfigLinearIterator_Next(t *testing.T) {
	var (
		min       = 10
		max       = 20
		stepCount = 3 // range [10, 15, 20]

		minFn       = func() int { return min }
		maxFn       = func() int { return max }
		stepCountFn = func() int { return stepCount }
	)

	iter := NewDynamicConfigLinearIterator(minFn, maxFn, stepCountFn, testlogger.New(t))

	t.Run("initialized values", func(t *testing.T) {
		// Start value - 15, next values - 20, 20
		assert.Equal(t, 20, iter.Next())
		assert.Equal(t, 20, iter.Next())
	})

	t.Run("changed max value", func(t *testing.T) {
		max = 30
		// range [10, 20, 30]
		// Start value - 20, next value - 30
		assert.Equal(t, 30, iter.Next())
	})

	t.Run("changed step count value", func(t *testing.T) {
		stepCount = 6
		// range [10, 14, 18, 22, 26, 30]
		// Start value - 18
		assert.Equal(t, 22, iter.Next())
		assert.Equal(t, 26, iter.Next())
		assert.Equal(t, 30, iter.Next())
	})
}

func TestDynamicConfigLinearIterator_Previous(t *testing.T) {
	var (
		min       = 10
		max       = 20
		stepCount = 3 // range [10, 15, 20]

		minFn       = func() int { return min }
		maxFn       = func() int { return max }
		stepCountFn = func() int { return stepCount }
	)

	iter := NewDynamicConfigLinearIterator(minFn, maxFn, stepCountFn, testlogger.New(t))

	t.Run("initialized values", func(t *testing.T) {
		// Start value - 15, previous values - 10, 10
		assert.Equal(t, 10, iter.Previous())
		assert.Equal(t, 10, iter.Previous())
	})

	t.Run("changed max value", func(t *testing.T) {
		max = 30
		// range [10, 20, 30]
		// Start value - 20, previous value - 10
		assert.Equal(t, 10, iter.Previous())
	})

	t.Run("changed step count value", func(t *testing.T) {
		stepCount = 6
		// range [10, 14, 18, 22, 26, 30]
		// Start value - 18
		assert.Equal(t, 14, iter.Previous())
		assert.Equal(t, 10, iter.Previous())
		assert.Equal(t, 10, iter.Previous())
	})
}

func TestDynamicConfigLinearIterator_Value(t *testing.T) {
	var (
		min       = 10
		max       = 20
		stepCount = 3 // range [10, 15, 20]

		minFn       = func() int { return min }
		maxFn       = func() int { return max }
		stepCountFn = func() int { return stepCount }
	)

	iter := NewDynamicConfigLinearIterator(minFn, maxFn, stepCountFn, testlogger.New(t))

	t.Run("initialized values", func(t *testing.T) {
		// range [10, 15, 20]
		assert.Equal(t, 15, iter.Value())
	})

	t.Run("changed max value", func(t *testing.T) {
		max = 30
		// range [10, 20, 30]
		assert.Equal(t, 20, iter.Value())
	})

	t.Run("changed step count value", func(t *testing.T) {
		stepCount = 6
		// range [10, 14, 18, 22, 26, 30]
		assert.Equal(t, 18, iter.Value())
	})
}

func TestDynamicConfigLinearIterator_Reset(t *testing.T) {
	var (
		min       = 10
		max       = 20
		stepCount = 3 // range [10, 15, 20]

		minFn       = func() int { return min }
		maxFn       = func() int { return max }
		stepCountFn = func() int { return stepCount }
	)

	iter := NewDynamicConfigLinearIterator(minFn, maxFn, stepCountFn, testlogger.New(t))

	t.Run("initialized values", func(t *testing.T) {
		// range [10, 15, 20]
		assert.Equal(t, 15, iter.Value())
		iter.Reset()
		assert.Equal(t, 15, iter.Value())
	})

	t.Run("changed max value", func(t *testing.T) {
		max = 30
		// range [10, 20, 30]
		assert.Equal(t, 20, iter.Value())
		iter.Reset()
		assert.Equal(t, 20, iter.Value())
	})

	t.Run("changed step count value", func(t *testing.T) {
		stepCount = 6
		// range [10, 14, 18, 22, 26, 30]
		assert.Equal(t, 18, iter.Value())
		iter.Reset()
		assert.Equal(t, 18, iter.Value())
	})

	t.Run("reached max and config not changed", func(t *testing.T) {
		// range [10, 14, 18, 22, 26, 30]
		assert.Equal(t, 22, iter.Next())
		assert.Equal(t, 26, iter.Next())
		assert.Equal(t, 30, iter.Next())

		iter.Reset()
		assert.Equal(t, 18, iter.Value())
	})

	t.Run("reached min and config not changed", func(t *testing.T) {
		// range [10, 14, 18, 22, 26, 30]
		assert.Equal(t, 14, iter.Previous())
		assert.Equal(t, 10, iter.Previous())

		iter.Reset()
		assert.Equal(t, 18, iter.Value())
	})
}
