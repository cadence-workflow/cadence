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

// LinearIterator is an Iterator that steps through a range linearly with a fixed step size.
// The fixed step size is calculated based on the range and the number of steps provided.
type LinearIterator[T Integer] struct {
	cur   int
	steps []T
}

// NewLinearIterator returns a new LinearIterator.
// The range is a range of integers from min to max. The stepCount is the number of steps between min and max (both inclusive).
// After creating the Iterator, the current value will be the middle value of the range.
// If stepCount is less or equal than 2 or the difference between min and max is less than 1, the Iterator will only have min and max.
//   - range [0, 10], 	stepCount 1 -> [0, 10]
//   - range [10, 10], 	stepCount 2 -> [10, 10]
//
// If the difference between min and max is less than stepCount, the Iterator will have all the values between min and max.
//   - range [0, 5], 	stepCount 10 -> [0, 1, 2, 3, 4, 5]
//   - range [0, 10], 	stepCount 11 -> [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
func NewLinearIterator[T Integer](min, max T, stepCount int) *LinearIterator[T] {
	// Ensure min <= max
	if min > max {
		min, max = max, min
	}

	totalRange := int(max - min)

	// If stepCount is less than 2 or the difference between min and max is less than 1,
	// return a Iterator with only min and max
	if stepCount <= 2 || totalRange <= 1 {
		return &LinearIterator[T]{
			cur:   0,
			steps: []T{min, max},
		}
	}

	// if the total range is less than the step count, return a Iterator with all the values
	if totalRange < stepCount {
		steps := make([]T, totalRange+1)

		steps[0], steps[totalRange] = min, max

		for i := 1; i < totalRange; i++ {
			steps[i] = min + T(i)
		}

		return &LinearIterator[T]{
			cur:   (len(steps) - 1) / 2,
			steps: steps,
		}
	}

	// Calculate step size as a float to ensure even distribution
	stepSize := float64(totalRange / (stepCount - 1))

	// Generate steps
	steps := make([]T, stepCount)
	for i := 0; i < stepCount; i++ {
		steps[i] = min + T(float64(i)*stepSize)
	}

	// Ensure the last value is exactly max (to avoid floating-point errors)
	steps[stepCount-1] = max

	return &LinearIterator[T]{
		cur:   (len(steps) - 1) / 2,
		steps: steps,
	}
}

// Next returns the next value in the range.
func (s *LinearIterator[T]) Next() T {
	if s.cur < len(s.steps)-1 {
		s.cur++
	}
	return s.steps[s.cur]
}

// Previous returns the previous value in the range.
func (s *LinearIterator[T]) Previous() T {
	if s.cur > 0 {
		s.cur--
	}
	return s.steps[s.cur]
}

// Value returns the current value in the range.
func (s *LinearIterator[T]) Value() T {
	return s.steps[s.cur]
}

// Reset resets the Iterator to the beginning of the range.
func (s *LinearIterator[T]) Reset() {
	s.cur = (len(s.steps) - 1) / 2
}
