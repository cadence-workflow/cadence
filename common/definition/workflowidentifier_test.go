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
	"github.com/stretchr/testify/suite"
)

// WorkflowIdentifierTestSuite is a test suite for the WorkflowIdentifier struct.
type WorkflowIdentifierTestSuite struct {
	suite.Suite
}

// SetupTest runs before each test function in the suite.
func (suite *WorkflowIdentifierTestSuite) SetupTest() {
	// Any necessary setup would go here, but it's not required in this case.
}

// TestSize verifies the Size method of WorkflowIdentifier.
func (suite *WorkflowIdentifierTestSuite) TestSize() {
	tests := []struct {
		wi       WorkflowIdentifier
		expected uint64
	}{
		{
			wi:       NewWorkflowIdentifier("domain", "workflow", "run"),
			expected: uint64(len("domain") + len("workflow") + len("run") + 3*16),
		},
		{
			wi:       NewWorkflowIdentifier("", "", ""),
			expected: uint64(3 * 16),
		},
		{
			wi:       NewWorkflowIdentifier("a", "b", "c"),
			expected: uint64(3 + 3*16),
		},
	}

	for _, test := range tests {
		size := test.wi.Size()
		assert.Equal(suite.T(), test.expected, size)
	}
}

// TestWorkflowIdentifierTestSuite runs the test suite.
func TestWorkflowIdentifierTestSuite(t *testing.T) {
	suite.Run(t, new(WorkflowIdentifierTestSuite))
}
