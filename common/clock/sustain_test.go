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

package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type check struct {
	seconds int
	value   bool
}

func TestSustain(t *testing.T) {
	cases := []struct {
		name     string
		duration time.Duration
		calls    []check
		expected []bool
	}{
		{
			name:     "simple case",
			duration: time.Second * 10,
			calls: []check{
				{0, true},
				{10, true},
			},
			expected: []bool{
				false,
				true,
			},
		},
		{
			name:     "intermediate successes",
			duration: 10 * time.Second,
			calls: []check{
				{0, true},
				{2, true},
				{2, true},
				{2, true},
				{2, true},
				{2, true},
			},
			expected: []bool{
				false,
				false,
				false,
				false,
				false,
				true,
			},
		},
		{
			name:     "resets after success",
			duration: time.Second * 10,
			calls: []check{
				{0, true},
				{10, true},
				{0, true},
			},
			expected: []bool{
				false,
				true,
				false,
			},
		},
		{
			name:     "resets after false",
			duration: time.Second * 10,
			calls: []check{
				{0, true},
				{1, false},
				{1, true},
				{9, true},
				{1, true},
			},
			expected: []bool{
				false,
				false,
				false,
				false,
				true,
			},
		},
		{
			name:     "duration = 0",
			duration: 0,
			calls: []check{
				{0, true},
			},
			expected: []bool{
				true,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clock := NewMockedTimeSource()
			sus := NewSustain(clock, func() time.Duration {
				return tc.duration
			})
			require.Equal(t, len(tc.calls), len(tc.expected))
			for i, c := range tc.calls {
				expected := tc.expected[i]
				clock.Advance(time.Duration(c.seconds) * time.Second)
				actual := sus.Check(c.value)
				assert.Equal(t, expected, actual, "check %d", i)
			}
		})
	}
}
