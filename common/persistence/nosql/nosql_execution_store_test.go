// Copyright (c) 2023 Uber Technologies, Inc.
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

package nosql

import (
	"context"
	"errors"
	"github.com/uber/cadence/common/types"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/service/history/constants"
)

func TestCreateWorkflowExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := nosqlplugin.NewMockDB(ctrl)
	logger := log.NewNoop()

	store := &nosqlExecutionStore{
		shardID:    1,
		nosqlStore: nosqlStore{logger: logger, db: mockDB},
	}

	ctx := context.Background()
	newWorkflowSnapshot := persistence.InternalWorkflowSnapshot{
		VersionHistories: &persistence.DataBlob{},
		ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
			DomainID:   constants.TestDomainID,
			WorkflowID: constants.TestWorkflowID,
			RunID:      constants.TestRunID,
		},
	}

	testCases := []struct {
		name             string
		request          *persistence.InternalCreateWorkflowExecutionRequest
		mockReturnError  error
		expectedResponse *persistence.CreateWorkflowExecutionResponse
		expectedError    error
	}{
		{
			name: "success",
			request: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID:                  123,
				Mode:                     persistence.CreateWorkflowModeBrandNew,
				PreviousRunID:            "previous-run-id",
				PreviousLastWriteVersion: 456,
				NewWorkflowSnapshot:      newWorkflowSnapshot,
			},
			mockReturnError:  nil,
			expectedResponse: &persistence.CreateWorkflowExecutionResponse{},
			expectedError:    nil,
		},
		{
			name: "failure - workflow already exists",
			request: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID:                  123,
				Mode:                     persistence.CreateWorkflowModeBrandNew,
				PreviousRunID:            "previous-run-id",
				PreviousLastWriteVersion: 456,
				NewWorkflowSnapshot:      newWorkflowSnapshot,
			},
			mockReturnError:  &persistence.WorkflowExecutionAlreadyStartedError{},
			expectedResponse: nil,
			expectedError:    &persistence.WorkflowExecutionAlreadyStartedError{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB.EXPECT().
				InsertWorkflowExecutionWithTasks(
					ctx,
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(tc.mockReturnError).Times(1)
			if tc.mockReturnError != nil {
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(errors.Is(tc.mockReturnError, &persistence.WorkflowExecutionAlreadyStartedError{})).AnyTimes()
				mockDB.EXPECT().IsTimeoutError(gomock.Any()).Return(errors.Is(tc.mockReturnError, &persistence.WorkflowExecutionAlreadyStartedError{})).AnyTimes()
				mockDB.EXPECT().IsThrottlingError(gomock.Any()).Return(errors.Is(tc.mockReturnError, &persistence.WorkflowExecutionAlreadyStartedError{})).AnyTimes()
				mockDB.EXPECT().IsDBUnavailableError(gomock.Any()).Return(errors.Is(tc.mockReturnError, &persistence.WorkflowExecutionAlreadyStartedError{})).AnyTimes()
			}

			_, err := store.CreateWorkflowExecution(ctx, tc.request)
			var internalServiceErr *types.InternalServiceError
			if errors.As(err, &internalServiceErr) {
				require.Equal(t, "CreateWorkflowExecution operation failed. Error: ", internalServiceErr.Message)
			}
		})
	}
}
