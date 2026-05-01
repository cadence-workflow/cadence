// Copyright (c) 2017 Uber Technologies, Inc.
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

package definition

import (
	"github.com/uber/cadence/common/constants"
)

type (
	// WorkflowIdentifier is the combinations which represent a workflow
	WorkflowIdentifier struct {
		DomainID   string
		WorkflowID string
		RunID      string
	}
)

// Size calculates the size in bytes of the WorkflowIdentifier struct.
func (wi *WorkflowIdentifier) ByteSize() uint64 {
	// Calculate the size of strings in bytes, we assume that all those fields are using ASCII which is 1 byte per char
	size := len(wi.DomainID) + len(wi.WorkflowID) + len(wi.RunID)
	// Each string internally holds a reference pointer and a length, which are 8 bytes each
	stringOverhead := 3 * constants.StringSizeOverheadBytes
	return uint64(size + stringOverhead)
}

// NewWorkflowIdentifier create a new WorkflowIdentifier
func NewWorkflowIdentifier(domainID string, workflowID string, runID string) WorkflowIdentifier {
	return WorkflowIdentifier{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}
}
