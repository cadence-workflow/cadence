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

import "golang.org/x/exp/constraints"

// Iterator is an interface for iterating through a range between two values.
// The range is a range of integers from a minimum to a maximum value (inclusive).
type Iterator[T Integer] interface {
	// Next returns the next value closer to the max value in the range
	// If the current value is the max value, Next will return the max value
	Next() T

	// Previous returns the previous value closer to the min value in the range
	// If the current value is the min value, Previous will return the min value
	Previous() T

	// Value returns the current value in the range
	Value() T

	// Reset resets the Iterator to its initial state
	Reset()
}

// Integer is a type constraint for the Iterator interface to ensure that only integer types are used.
type Integer = constraints.Integer
