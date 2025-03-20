// Copyright (c) 2017-2020 Uber Technologies, Inc.
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

package task

import (
	"testing"
)

func TestShouldRedactContextHeader(t *testing.T) {
	cases := []struct {
		key             string
		hiddenValueKeys map[string]interface{}
		expected        bool
	}{
		{
			key: "key1",
			hiddenValueKeys: map[string]interface{}{
				"key1": true,
			},
			expected: true,
		},
		{
			key: "key2",
			hiddenValueKeys: map[string]interface{}{
				"key1": true,
			},
			expected: false,
		},
		{
			key:             "key3",
			hiddenValueKeys: map[string]interface{}{},
			expected:        false,
		},
		{
			key:             "key4",
			hiddenValueKeys: nil,
			expected:        false,
		},
		{
			key: "key5",
			hiddenValueKeys: map[string]interface{}{
				"key5": "true",
			},
			expected: true,
		},
	}

	for _, c := range cases {
		result := shouldRedactContextHeader(c.key, c.hiddenValueKeys)
		if result != c.expected {
			t.Errorf("shouldRedactContextHeader(%s, %v) = %t; expected %t", c.key, c.hiddenValueKeys, result, c.expected)
		}
	}
}
