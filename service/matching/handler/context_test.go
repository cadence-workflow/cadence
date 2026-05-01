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

package handler

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

func TestHandleErrNil(t *testing.T) {
	reqCtx := &handlerContext{
		scope:  metrics.NoopScope,
		logger: log.NewNoop(),
	}

	err := reqCtx.handleErr(nil)
	assert.NoError(t, err)
}

func TestHandleErrKnowErrors(t *testing.T) {
	testCases := []struct {
		name    string
		err     error
		wantErr error
	}{
		{
			name: "InternalServiceError",
			err:  &types.InternalServiceError{},
		},
		{
			name: "BadRequestError",
			err:  &types.BadRequestError{},
		},
		{
			name: "EntityNotExistsError",
			err:  &types.EntityNotExistsError{},
		},
		{
			name: "WorkflowExecutionAlreadyStartedError",
			err:  &types.WorkflowExecutionAlreadyStartedError{},
		},
		{
			name: "DomainAlreadyExistsError",
			err:  &types.DomainAlreadyExistsError{},
		},
		{
			name: "QueryFailedError",
			err:  &types.QueryFailedError{},
		},
		{
			name: "LimitExceededError",
			err:  &types.LimitExceededError{},
		},
		{
			name: "ServiceBusyError",
			err:  &types.ServiceBusyError{},
		},
		{
			name: "DomainNotActiveError",
			err:  &types.DomainNotActiveError{},
		},
		{
			name: "RemoteSyncMatchedError",
			err:  &types.RemoteSyncMatchedError{},
		},
		{
			name: "StickyWorkerUnavailableError",
			err:  &types.StickyWorkerUnavailableError{},
		},
		{
			name: "ReadOnlyPartitionError",
			err:  &types.ReadOnlyPartitionError{},
		},
		{
			name: "TaskListNotOwnedByHostError",
			err:  &cadence_errors.TaskListNotOwnedByHostError{},
		},
	}

	for _, tc := range testCases {
		reqCtx := &handlerContext{
			scope:  metrics.NoopScope,
			logger: log.NewNoop(),
		}

		t.Run(tc.name, func(t *testing.T) {
			handledErr := reqCtx.handleErr(tc.err)
			require.Equal(t, tc.err, handledErr)

			wrappedErr := fmt.Errorf("wrapped: %w", tc.err)
			wrappedHandledErr := reqCtx.handleErr(wrappedErr)
			require.Equal(t, wrappedErr, wrappedHandledErr)
		})
	}
}

func TestHandleErrUncategorizedError(t *testing.T) {
	reqCtx := &handlerContext{
		scope:  metrics.NoopScope,
		logger: log.NewNoop(),
	}

	err := reqCtx.handleErr(assert.AnError)
	assert.ErrorAs(t, err, new(*types.InternalServiceError))
}
