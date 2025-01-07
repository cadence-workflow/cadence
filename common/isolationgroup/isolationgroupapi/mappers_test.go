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

package isolationgroupapi

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapAllIsolationGroupStates(t *testing.T) {

	tests := map[string]struct {
		in          []interface{}
		expected    []string
		expectedErr error
	}{
		"valid mapping": {
			in:       []interface{}{"zone-1", "zone-2", "zone-3"},
			expected: []string{"zone-1", "zone-2", "zone-3"},
		},
		"invalid mapping": {
			in:          []interface{}{1, 2, 3},
			expectedErr: errors.New("failed to get all-isolation-groups response from dynamic config: got 1 (int)"),
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			res, err := MapAllIsolationGroupsResponse(td.in)
			assert.Equal(t, td.expected, res)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}
