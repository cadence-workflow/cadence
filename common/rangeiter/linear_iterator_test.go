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
)

func TestNewLinearIterator(t *testing.T) {
	for name, c := range map[string]struct {
		min, max  int
		stepCount int
		want      *LinearIterator[int]
	}{
		"min greater max, stepCount greater range": {
			min:       10,
			max:       1,
			stepCount: 10, // range: 10 - 1 = 9
			want: &LinearIterator[int]{
				cur:   4,
				steps: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
		},
		"max greater min, stepCount is one": {
			min:       0,
			max:       10,
			stepCount: 1,
			want: &LinearIterator[int]{
				cur:   0,
				steps: []int{0, 10},
			},
		},
		"max greater min, stepCount is two": {
			min:       0,
			max:       10,
			stepCount: 2,
			want: &LinearIterator[int]{
				cur:   0,
				steps: []int{0, 10},
			},
		},
		"max greater min, stepCount is three": {
			min:       0,
			max:       10,
			stepCount: 3,
			want: &LinearIterator[int]{
				cur:   1,
				steps: []int{0, 5, 10},
			},
		},
		"max greater min, stepCount is four": {
			min:       0,
			max:       10,
			stepCount: 4,
			want: &LinearIterator[int]{
				cur:   1,
				steps: []int{0, 3, 6, 10},
			},
		},
		"max greater min, stepCount is six": {
			min:       0,
			max:       10,
			stepCount: 6,
			want: &LinearIterator[int]{
				cur:   2,
				steps: []int{0, 2, 4, 6, 8, 10},
			},
		},
		"max greater min than 1, stepCount is greater range": {
			min:       9,
			max:       10,
			stepCount: 10,
			want: &LinearIterator[int]{
				cur:   0,
				steps: []int{9, 10},
			},
		},
		"max equal mim, stepCount is greater range": {
			min:       10,
			max:       10,
			stepCount: 10,
			want: &LinearIterator[int]{
				cur:   0,
				steps: []int{10, 10},
			},
		},
		"max greater min, range is less than stepCount": {
			min:       0,
			max:       10,
			stepCount: 20, // range: 10 - 0 = 10
			want: &LinearIterator[int]{
				cur:   5,
				steps: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
		},
		"max greater min, range is equal to stepCount": {
			min:       0,
			max:       10,
			stepCount: 10, // range: 10 - 0 = 10
			want: &LinearIterator[int]{
				cur:   4,
				steps: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 10},
			},
		},
		"negative max and min, stepCount is two": {
			min:       -1000 * 1000,
			max:       -1000,
			stepCount: 35,
			want: &LinearIterator[int]{
				cur:   17,
				steps: []int{-1000000, -970618, -941236, -911854, -882472, -853090, -823708, -794326, -764944, -735562, -706180, -676798, -647416, -618034, -588652, -559270, -529888, -500506, -471124, -441742, -412360, -382978, -353596, -324214, -294832, -265450, -236068, -206686, -177304, -147922, -118540, -89158, -59776, -30394, -1000},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := NewLinearIterator(c.min, c.max, c.stepCount)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestLinearIterator_Next(t *testing.T) {
	for name, c := range map[string]struct {
		cur   int
		steps []int

		wantValue int
		wantCur   int
	}{
		"cur equals to min": {
			cur:   0,
			steps: []int{0, 1, 2, 3, 4, 5},

			wantValue: 1,
			wantCur:   1,
		},
		"cur equals to middle": {
			cur:   3, // value - 3
			steps: []int{0, 1, 2, 3, 4, 5},

			wantValue: 4,
			wantCur:   4,
		},
		"cur equals to max": {
			cur:   5,
			steps: []int{0, 1, 2, 3, 4, 5},

			wantValue: 5,
			wantCur:   5,
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := &LinearIterator[int]{
				cur:   c.cur,
				steps: c.steps,
			}
			assert.Equal(t, c.wantValue, s.Next())
			assert.Equal(t, c.wantCur, s.cur)
		})
	}
}

func TestLinearIterator_Previous(t *testing.T) {
	for name, c := range map[string]struct {
		cur   int
		steps []int

		wantValue int
		wantCur   int
	}{
		"cur equals to min": {
			cur:   0,
			steps: []int{0, 1, 2, 3, 4, 5},

			wantValue: 0,
			wantCur:   0,
		},
		"cur equals to middle": {
			cur:   3, // value - 3
			steps: []int{0, 1, 2, 3, 4, 5},

			wantValue: 2,
			wantCur:   2,
		},
		"cur equals to max": {
			cur:   6,
			steps: []int{0, 1, 2, 3, 4, 5},

			wantValue: 5,
			wantCur:   5,
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := &LinearIterator[int]{
				cur:   c.cur,
				steps: c.steps,
			}
			assert.Equal(t, c.wantValue, s.Previous())
			assert.Equal(t, c.wantCur, s.cur)
		})
	}
}

func TestLinearIterator_Reset(t *testing.T) {
	for name, c := range map[string]struct {
		cur   int
		steps []int

		wantValue int
		wantCur   int
	}{
		"cur equals to min": {
			cur:   0,
			steps: []int{0, 1, 2, 3, 4, 5},

			wantValue: 2,
			wantCur:   2,
		},
		"cur equals to middle": {
			cur:   3, // value - 3
			steps: []int{0, 1, 2, 3, 4, 5},

			wantValue: 2,
			wantCur:   2,
		},
		"cur equals to max": {
			cur:   6,
			steps: []int{0, 1, 2, 3, 4, 5},

			wantValue: 2,
			wantCur:   2,
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := &LinearIterator[int]{
				cur:   c.cur,
				steps: c.steps,
			}

			s.Reset()
			assert.Equal(t, c.wantValue, s.Value())
			assert.Equal(t, c.wantCur, s.cur)
		})
	}
}

func TestLinearIterator_Value(t *testing.T) {
	Iterator := &LinearIterator[int]{
		cur:   3,
		steps: []int{0, 1, 2, 3, 4, 5},
	}
	assert.Equal(t, 3, Iterator.Value())
}
