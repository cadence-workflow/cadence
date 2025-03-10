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

package definition

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWorkflowIdentifierSize verifies the Size method of WorkflowIdentifier.
func TestWorkflowIdentifierSize(t *testing.T) {
	tests := []struct {
		name     string
		wi       WorkflowIdentifier
		expected uint64
	}{
		{
			name:     "non-empty fields",
			wi:       NewWorkflowIdentifier("domain", "workflow", "run"),
			expected: uint64(len("domain") + len("workflow") + len("run") + 3*16),
		},
		{
			name:     "empty fields",
			wi:       NewWorkflowIdentifier("", "", ""),
			expected: uint64(3 * 16),
		},
		{
			name:     "short fields",
			wi:       NewWorkflowIdentifier("a", "b", "c"),
			expected: uint64(3 + 3*16),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			size := test.wi.Size()
			assert.Equal(t, test.expected, size)
		})
	}
}
