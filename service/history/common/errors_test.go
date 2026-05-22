// Copyright (c) 2020 Uber Technologies, Inc.
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
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestIsOperationPossiblySuccessfulError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"WorkflowExecutionAlreadyStartedError (types)", &types.WorkflowExecutionAlreadyStartedError{}, false},
		{"WorkflowExecutionAlreadyStartedError (persistence)", &persistence.WorkflowExecutionAlreadyStartedError{}, false},
		{"CurrentWorkflowConditionFailedError", &persistence.CurrentWorkflowConditionFailedError{}, false},
		{"ConditionFailedError", &persistence.ConditionFailedError{}, false},
		{"ServiceBusyError", &types.ServiceBusyError{}, false},
		{"LimitExceededError", &types.LimitExceededError{}, false},
		{"ShardOwnershipLostError", &persistence.ShardOwnershipLostError{}, false},
		{"TimeoutError", &persistence.TimeoutError{}, true},
		{"context.DeadlineExceeded", context.DeadlineExceeded, true},
		{"generic error", assert.AnError, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsOperationPossiblySuccessfulError(tc.err))
		})
	}
}
