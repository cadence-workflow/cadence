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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
)

func TestCreateWorkflowExecution(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		setupMock     func(*nosqlplugin.MockDB, int) // Now accepts shard ID as parameter
		expectedResp  *persistence.CreateWorkflowExecutionResponse
		expectedError error
	}{
		{
			name: "success",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedResp:  &persistence.CreateWorkflowExecutionResponse{},
			expectedError: nil,
		},
		{
			name: "ShardRangeIDNotMatch condition failure",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				err := &nosqlplugin.WorkflowOperationConditionFailure{
					ShardRangeIDNotMatch: common.Int64Ptr(456),
				}
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(err)
			},
			expectedResp: nil, // No response expected on error
			expectedError: &persistence.ShardOwnershipLostError{
				ShardID: 123,
				Msg:     "Failed to create workflow execution.  Request RangeID: 123, Actual RangeID: 456",
			},
		},
		{
			name: "WorkflowExecutionAlreadyExists condition failure",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				err := &nosqlplugin.WorkflowOperationConditionFailure{
					WorkflowExecutionAlreadyExists: &nosqlplugin.WorkflowExecutionAlreadyExists{
						OtherInfo:        "Workflow with ID already exists",
						CreateRequestID:  "existing-request-id",
						RunID:            "existing-run-id",
						State:            persistence.WorkflowStateCompleted,
						CloseStatus:      persistence.WorkflowCloseStatusCompleted,
						LastWriteVersion: 123,
					},
				}
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(err)
			},
			expectedResp: nil, // No response expected on error
			expectedError: &persistence.WorkflowExecutionAlreadyStartedError{
				Msg:              "Workflow with ID already exists",
				StartRequestID:   "existing-request-id",
				RunID:            "existing-run-id",
				State:            persistence.WorkflowStateCompleted,
				CloseStatus:      persistence.WorkflowCloseStatusCompleted,
				LastWriteVersion: 123,
			},
		},
		{
			name: "Unknown condition failure",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				err := &nosqlplugin.WorkflowOperationConditionFailure{
					UnknownConditionFailureDetails: common.StringPtr("Unknown error occurred during operation"),
				}
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(err)
			},
			expectedResp: nil,
			expectedError: &persistence.ShardOwnershipLostError{
				ShardID: 123,
				Msg:     "Unknown error occurred during operation",
			},
		},
		{
			name: "Current workflow condition failure",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				err := &nosqlplugin.WorkflowOperationConditionFailure{
					CurrentWorkflowConditionFailInfo: common.StringPtr("Current workflow condition failed"),
				}
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(err)
			},
			expectedResp: nil,
			expectedError: &persistence.CurrentWorkflowConditionFailedError{
				Msg: "Current workflow condition failed",
			},
		},
		{
			name: "Unexpected error type leading to unsupported condition failure",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				// Simulate returning an unexpected error type from the mock
				unexpectedErr := errors.New("unexpected error type")
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(unexpectedErr)
			},
			expectedResp:  nil,
			expectedError: fmt.Errorf("unsupported conditionFailureReason error"), // Expected generic error for unexpected conditions
		},
		{
			name: "Duplicate request error",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&nosqlplugin.WorkflowOperationConditionFailure{
						DuplicateRequest: &nosqlplugin.DuplicateRequest{
							RequestType: persistence.WorkflowRequestTypeSignal,
							RunID:       "test-run-id",
						},
					})
			},
			expectedResp: nil,
			expectedError: &persistence.DuplicateRequestError{
				RequestType: persistence.WorkflowRequestTypeSignal,
				RunID:       "test-run-id",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockDB := nosqlplugin.NewMockDB(controller)
			store := newTestNosqlExecutionStore(mockDB, log.NewNoop())
			shardID := store.GetShardID()

			tc.setupMock(mockDB, shardID)

			resp, err := store.CreateWorkflowExecution(ctx, newCreateWorkflowExecutionRequest())

			if diff := cmp.Diff(tc.expectedResp, resp); diff != "" {
				t.Errorf("CreateWorkflowExecution() response mismatch (-want +got):\n%s", diff)
			}
			if tc.expectedError != nil {
				require.ErrorAs(t, err, &tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUpdateWorkflowExecution(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		setupMock     func(*nosqlplugin.MockDB, int)
		request       func() *persistence.InternalUpdateWorkflowExecutionRequest
		expectedError error // Ensure we are using `expectedError` consistently
	}{

		{
			name: "success",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				mockDB.EXPECT().
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), nil, gomock.Any(), gomock.Any()).
					Return(nil)
			},
			request:       newUpdateWorkflowExecutionRequest,
			expectedError: nil,
		},
		{
			name: "Success - UpdateWorkflowModeIgnoreCurrent",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				mockDB.EXPECT().
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), nil, gomock.Any(), gomock.Any()).
					Return(nil)
			},
			request: func() *persistence.InternalUpdateWorkflowExecutionRequest {
				req := newUpdateWorkflowExecutionRequest()
				req.Mode = persistence.UpdateWorkflowModeIgnoreCurrent
				return req
			},
			expectedError: nil,
		},
		{
			name: "Duplicate request error",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				mockDB.EXPECT().
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), nil, gomock.Any(), gomock.Any()).
					Return(&nosqlplugin.WorkflowOperationConditionFailure{
						DuplicateRequest: &nosqlplugin.DuplicateRequest{
							RequestType: persistence.WorkflowRequestTypeSignal,
							RunID:       "test-run-id",
						},
					})
			},
			request: newUpdateWorkflowExecutionRequest,
			expectedError: &persistence.DuplicateRequestError{
				RequestType: persistence.WorkflowRequestTypeSignal,
				RunID:       "test-run-id",
			},
		},
		{
			name: "UpdateWorkflowModeBypassCurrent - assertNotCurrentExecution failure",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				mockDB.EXPECT().
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), nil, gomock.Any(), gomock.Any()).
					Times(0)
			},
			request: func() *persistence.InternalUpdateWorkflowExecutionRequest {
				req := newUpdateWorkflowExecutionRequest()
				req.Mode = persistence.UpdateWorkflowModeBypassCurrent
				return req
			},
			expectedError: &types.InternalServiceError{Message: "assertNotCurrentExecution failure"},
		},
		{
			name: "Unknown update mode",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
			},
			request: func() *persistence.InternalUpdateWorkflowExecutionRequest {
				req := newUpdateWorkflowExecutionRequest()
				req.Mode = -1
				return req
			},
			expectedError: &types.InternalServiceError{Message: "UpdateWorkflowExecution: unknown mode: -1"},
		},
		{
			name:      "Bypass_current_execution_failure_due_to_assertNotCurrentExecution",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {},
			request: func() *persistence.InternalUpdateWorkflowExecutionRequest {
				req := newUpdateWorkflowExecutionRequest()
				req.Mode = persistence.UpdateWorkflowModeBypassCurrent
				return req
			},
			expectedError: &types.InternalServiceError{Message: "assertNotCurrentExecution failure"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			mockDB := nosqlplugin.NewMockDB(controller)
			mockTaskSerializer := serialization.NewMockTaskSerializer(controller)
			store, _ := NewExecutionStore(1, mockDB, log.NewNoop(), mockTaskSerializer, &persistence.DynamicConfiguration{
				EnableHistoryTaskDualWriteMode: func(...dynamicconfig.FilterOption) bool { return false },
			})

			tc.setupMock(mockDB, 1)

			req := tc.request()
			err := store.UpdateWorkflowExecution(ctx, req)
			if tc.expectedError != nil {
				require.Error(t, err)
				require.IsType(t, tc.expectedError, err, "Error type does not match the expected one.")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNosqlExecutionStore(t *testing.T) {
	ctx := context.Background()
	shardID := 1
	testCases := []struct {
		name          string
		setupMock     func(*gomock.Controller) *nosqlExecutionStore
		testFunc      func(*nosqlExecutionStore) error
		expectedError error
	}{
		{
			name: "CreateWorkflowExecution success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.CreateWorkflowExecution(ctx, newCreateWorkflowExecutionRequest())
				return err
			},
			expectedError: nil,
		},
		{
			name: "CreateWorkflowExecution failure - workflow already exists",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&persistence.WorkflowExecutionAlreadyStartedError{}).Times(1)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsTimeoutError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsThrottlingError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsDBUnavailableError(gomock.Any()).Return(false).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.CreateWorkflowExecution(ctx, newCreateWorkflowExecutionRequest())
				return err
			},
			expectedError: &persistence.WorkflowExecutionAlreadyStartedError{},
		},
		{
			name: "CreateWorkflowExecution failure - shard ownership lost",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&persistence.ShardOwnershipLostError{ShardID: shardID, Msg: "shard ownership lost"}).Times(1)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsTimeoutError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsThrottlingError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsDBUnavailableError(gomock.Any()).Return(false).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.CreateWorkflowExecution(ctx, newCreateWorkflowExecutionRequest())
				return err
			},
			expectedError: &persistence.ShardOwnershipLostError{},
		},
		{
			name: "CreateWorkflowExecution failure - current workflow condition failed",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&persistence.CurrentWorkflowConditionFailedError{Msg: "current workflow condition failed"}).Times(1)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsTimeoutError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsThrottlingError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsDBUnavailableError(gomock.Any()).Return(false).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.CreateWorkflowExecution(ctx, newCreateWorkflowExecutionRequest())
				return err
			},
			expectedError: &persistence.CurrentWorkflowConditionFailedError{},
		},
		{
			name: "CreateWorkflowExecution failure - generic internal service error",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.InternalServiceError{Message: "generic internal service error"}).Times(1)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsTimeoutError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsThrottlingError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsDBUnavailableError(gomock.Any()).Return(false).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.CreateWorkflowExecution(ctx, newCreateWorkflowExecutionRequest())
				return err
			},
			expectedError: &types.InternalServiceError{},
		},
		{
			name: "CreateWorkflowExecution failure - duplicate request error",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&nosqlplugin.WorkflowOperationConditionFailure{
						DuplicateRequest: &nosqlplugin.DuplicateRequest{
							RunID: "abc",
						},
					}).Times(1)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsTimeoutError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsThrottlingError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsDBUnavailableError(gomock.Any()).Return(false).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.CreateWorkflowExecution(ctx, newCreateWorkflowExecutionRequest())
				return err
			},
			expectedError: &persistence.DuplicateRequestError{},
		},
		{
			name: "GetWorkflowExecution success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					SelectWorkflowExecution(ctx, shardID, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&nosqlplugin.WorkflowExecution{}, nil).Times(1)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.GetWorkflowExecution(ctx, newGetWorkflowExecutionRequest())
				return err
			},
			expectedError: nil,
		},
		{
			name: "GetWorkflowExecution failure - not found",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					SelectWorkflowExecution(ctx, shardID, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, &types.EntityNotExistsError{}).Times(1)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.GetWorkflowExecution(ctx, newGetWorkflowExecutionRequest())
				return err
			},
			expectedError: &types.EntityNotExistsError{},
		},
		{
			name: "UpdateWorkflowExecution success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Nil(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				err := store.UpdateWorkflowExecution(ctx, newUpdateWorkflowExecutionRequest())
				return err
			},
			expectedError: nil,
		},
		{
			name: "UpdateWorkflowExecution failure - invalid update mode",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				// No operation expected on the DB due to invalid mode
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				request := newUpdateWorkflowExecutionRequest()
				request.Mode = persistence.UpdateWorkflowMode(-1)
				return store.UpdateWorkflowExecution(ctx, request)
			},
			expectedError: &types.InternalServiceError{},
		},
		{
			name: "UpdateWorkflowExecution failure - condition not met",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				conditionFailure := &nosqlplugin.WorkflowOperationConditionFailure{
					UnknownConditionFailureDetails: common.StringPtr("condition not met"),
				}
				mockDB.EXPECT().
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Nil(), gomock.Any(), gomock.Any()).
					Return(conditionFailure).Times(1)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.UpdateWorkflowExecution(ctx, newUpdateWorkflowExecutionRequest())
			},
			expectedError: &persistence.ConditionFailedError{},
		},
		{
			name: "UpdateWorkflowExecution failure - operational error",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Nil(), gomock.Any(), gomock.Any()).
					Return(errors.New("database is unavailable")).Times(1)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.UpdateWorkflowExecution(ctx, newUpdateWorkflowExecutionRequest())
			},
			expectedError: &types.InternalServiceError{Message: "database is unavailable"},
		},
		{
			name: "DeleteWorkflowExecution success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					DeleteWorkflowExecution(ctx, shardID, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.DeleteWorkflowExecution(ctx, &persistence.DeleteWorkflowExecutionRequest{
					DomainID:   "domainID",
					WorkflowID: "workflowID",
					RunID:      "runID",
				})
			},
			expectedError: nil,
		},
		{
			name: "DeleteWorkflowExecution failure - workflow does not exist",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					DeleteWorkflowExecution(ctx, shardID, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.EntityNotExistsError{Message: "workflow does not exist"})
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.DeleteWorkflowExecution(ctx, &persistence.DeleteWorkflowExecutionRequest{
					DomainID:   "domainID",
					WorkflowID: "workflowID",
					RunID:      "runID",
				})
			},
			expectedError: &types.EntityNotExistsError{Message: "workflow does not exist"},
		},
		{
			name: "DeleteCurrentWorkflowExecution success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					DeleteCurrentWorkflow(ctx, shardID, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.DeleteCurrentWorkflowExecution(ctx, &persistence.DeleteCurrentWorkflowExecutionRequest{
					DomainID:   "domainID",
					WorkflowID: "workflowID",
					RunID:      "runID",
				})
			},
			expectedError: nil,
		},
		{
			name: "DeleteCurrentWorkflowExecution failure - current workflow does not exist",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					DeleteCurrentWorkflow(ctx, shardID, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.EntityNotExistsError{Message: "current workflow does not exist"})
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.DeleteCurrentWorkflowExecution(ctx, &persistence.DeleteCurrentWorkflowExecutionRequest{
					DomainID:   "domainID",
					WorkflowID: "workflowID",
					RunID:      "runID",
				})
			},
			expectedError: &types.EntityNotExistsError{Message: "current workflow does not exist"},
		},
		{
			name: "ListCurrentExecutions success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					SelectAllCurrentWorkflows(ctx, shardID, gomock.Any(), gomock.Any()).
					Return([]*persistence.CurrentWorkflowExecution{}, nil, nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.ListCurrentExecutions(ctx, &persistence.ListCurrentExecutionsRequest{})
				return err
			},
			expectedError: nil,
		},
		{
			name: "ListCurrentExecutions failure - database error",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					SelectAllCurrentWorkflows(ctx, shardID, gomock.Any(), gomock.Any()).
					Return(nil, nil, errors.New("database error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.ListCurrentExecutions(ctx, &persistence.ListCurrentExecutionsRequest{})
				return err
			},
			expectedError: &types.InternalServiceError{Message: "database error"},
		},
		{
			name: "ListConcreteExecutions success - has executions",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				// Corrected return value type to match expected method signature
				executions := []*persistence.InternalListConcreteExecutionsEntity{
					{
						ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
							WorkflowID: "workflowID",
							RunID:      "runID",
						},
					},
				}
				mockDB.EXPECT().
					SelectAllWorkflowExecutions(ctx, shardID, gomock.Any(), gomock.Any()).
					Return(executions, nil, nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				resp, err := store.ListConcreteExecutions(ctx, &persistence.ListConcreteExecutionsRequest{})
				if err != nil {
					return err
				}
				if len(resp.Executions) == 0 {
					return errors.New("expected to find executions")
				}
				return nil
			},
			expectedError: nil,
		},
		{
			name: "ListConcreteExecutions failure - database error",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					SelectAllWorkflowExecutions(ctx, shardID, gomock.Any(), gomock.Any()).
					Return(nil, nil, errors.New("database error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.ListConcreteExecutions(ctx, &persistence.ListConcreteExecutionsRequest{})
				return err
			},
			expectedError: &types.InternalServiceError{Message: "database error"},
		},
		{
			name: "GetTransferTasks success - has tasks",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				tasks := []*nosqlplugin.TransferTask{{TaskID: 1}}
				mockDB.EXPECT().
					SelectTransferTasksOrderByTaskID(ctx, shardID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(tasks, nil, nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				resp, err := store.GetTransferTasks(ctx, &persistence.GetTransferTasksRequest{})
				if err != nil {
					return err
				}
				if len(resp.Tasks) == 0 {
					return errors.New("expected to find transfer tasks")
				}
				return nil
			},
			expectedError: nil,
		},
		{
			name: "GetReplicationTasks success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				tasks := []*nosqlplugin.ReplicationTask{{TaskID: 1}}
				mockDB.EXPECT().
					SelectReplicationTasksOrderByTaskID(ctx, shardID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(tasks, nil, nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				resp, err := store.GetReplicationTasks(ctx, &persistence.GetReplicationTasksRequest{})
				if err != nil {
					return err
				}
				if len(resp.Tasks) == 0 {
					return errors.New("expected to find replication tasks")
				}
				return nil
			},
			expectedError: nil,
		},
		{
			name: "GetReplicationTasks success - empty task list",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					SelectReplicationTasksOrderByTaskID(ctx, shardID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*nosqlplugin.ReplicationTask{}, nil, nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				resp, err := store.GetReplicationTasks(ctx, &persistence.GetReplicationTasksRequest{})
				if err != nil {
					return err
				}
				if len(resp.Tasks) != 0 {
					return errors.New("expected no replication tasks")
				}
				return nil
			},
			expectedError: nil,
		},
		{
			name: "CompleteTransferTask success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					DeleteTransferTask(ctx, shardID, gomock.Any()).
					Return(nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.CompleteTransferTask(ctx, &persistence.CompleteTransferTaskRequest{TaskID: 1})
			},
			expectedError: nil,
		},
		{
			name: "CompleteTransferTask with zero TaskID",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)

				mockDB.EXPECT().
					DeleteTransferTask(ctx, shardID, int64(0)).
					Return(nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.CompleteTransferTask(ctx, &persistence.CompleteTransferTaskRequest{TaskID: 0})
			},
			expectedError: nil,
		},
		{
			name: "CompleteReplicationTask success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					DeleteReplicationTask(ctx, shardID, int64(1)).
					Return(nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.CompleteReplicationTask(ctx, &persistence.CompleteReplicationTaskRequest{TaskID: 1})
			},
			expectedError: nil,
		},
		{
			name: "CompleteReplicationTask failure - database error",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					DeleteReplicationTask(ctx, shardID, int64(1)).
					Return(errors.New("database error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.CompleteReplicationTask(ctx, &persistence.CompleteReplicationTaskRequest{TaskID: 1})
			},
			expectedError: &types.InternalServiceError{Message: "database error"},
		},
		{
			name: "CompleteReplicationTask with non-existent task ID",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				mockDB.EXPECT().
					DeleteReplicationTask(ctx, shardID, int64(9999)).
					Return(&types.EntityNotExistsError{Message: "replication task does not exist"}) // Simulate task not found
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.CompleteReplicationTask(ctx, &persistence.CompleteReplicationTaskRequest{TaskID: 9999})
			},
			expectedError: &types.EntityNotExistsError{Message: "replication task does not exist"},
		},
		{
			name: "CompleteTimerTask success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					DeleteTimerTask(ctx, shardID, int64(1), gomock.Any()).
					Return(nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.CompleteTimerTask(ctx, &persistence.CompleteTimerTaskRequest{
					TaskID:              1,
					VisibilityTimestamp: time.Now(),
				})
			},
			expectedError: nil,
		},
		{
			name: "CompleteTimerTask failure - database error",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					DeleteTimerTask(ctx, shardID, int64(1), gomock.Any()).
					Return(errors.New("database error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.CompleteTimerTask(ctx, &persistence.CompleteTimerTaskRequest{
					TaskID:              1,
					VisibilityTimestamp: time.Now(),
				})
			},
			expectedError: &types.InternalServiceError{Message: "database error"},
		},
		{
			name: "CompleteTimerTask with future VisibilityTimestamp",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					DeleteTimerTask(ctx, shardID, int64(1), gomock.Any()). // Expect the call with any timestamp
					Return(nil)                                            // Assuming successful deletion
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				return store.CompleteTimerTask(ctx, &persistence.CompleteTimerTaskRequest{
					TaskID:              1,
					VisibilityTimestamp: time.Now().Add(24 * time.Hour), // Future timestamp
				})
			},
			expectedError: nil, // Adjust based on actual behavior
		},
		{
			name: "GetTimerIndexTasks success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					SelectTimerTasksOrderByVisibilityTime(
						ctx,
						shardID,
						10,
						gomock.Nil(),
						gomock.Any(),
						gomock.Any(),
					).Return([]*persistence.TimerTaskInfo{}, nil, nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.GetTimerIndexTasks(ctx, &persistence.GetTimerIndexTasksRequest{
					BatchSize:    10,
					MinTimestamp: time.Now().Add(-time.Hour),
					MaxTimestamp: time.Now(),
				})
				return err
			},
			expectedError: nil,
		},
		{
			name: "GetTimerIndexTasks success - empty result",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					SelectTimerTasksOrderByVisibilityTime(ctx, shardID, 10, gomock.Nil(), gomock.Any(), gomock.Any()).
					Return([]*persistence.TimerTaskInfo{}, []byte{}, nil) // Return an empty list
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				resp, err := store.GetTimerIndexTasks(ctx, &persistence.GetTimerIndexTasksRequest{
					BatchSize:    10,
					MinTimestamp: time.Now().Add(-time.Hour),
					MaxTimestamp: time.Now(),
				})
				if err != nil {
					return err
				}
				if len(resp.Timers) != 0 {
					return errors.New("expected empty result set for timers")
				}
				return nil
			},
			expectedError: nil,
		},
		{
			name: "PutReplicationTaskToDLQ success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				replicationTaskInfo := newInternalReplicationTaskInfo()

				mockDB.EXPECT().
					InsertReplicationDLQTask(ctx, shardID, "sourceCluster", &nosqlplugin.HistoryMigrationTask{
						Replication: &replicationTaskInfo,
						Task:        nil,
					}).Return(nil)

				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				taskInfo := newInternalReplicationTaskInfo()
				return store.PutReplicationTaskToDLQ(ctx, &persistence.InternalPutReplicationTaskToDLQRequest{
					SourceClusterName: "sourceCluster",
					TaskInfo:          &taskInfo,
				})
			},
			expectedError: nil,
		},
		{
			name: "GetTimerIndexTasks failure - database error",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				mockDB.EXPECT().
					SelectTimerTasksOrderByVisibilityTime(ctx, shardID, 10, gomock.Nil(), gomock.Any(), gomock.Any()).
					Return(nil, nil, errors.New("database error"))
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.GetTimerIndexTasks(ctx, &persistence.GetTimerIndexTasksRequest{
					BatchSize:    10,
					MinTimestamp: time.Now().Add(-time.Hour),
					MaxTimestamp: time.Now(),
				})
				return err
			},
			expectedError: &types.InternalServiceError{Message: "database error"},
		},
		{
			name: "GetReplicationTasksFromDLQ success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)

				nextPageToken := []byte("next-page-token")
				replicationTasks := []*persistence.InternalReplicationTaskInfo{}
				mockDB.EXPECT().
					SelectReplicationDLQTasksOrderByTaskID(
						ctx,
						shardID,
						"sourceCluster",
						10,
						gomock.Any(),
						int64(0),
						int64(100),
					).Return(replicationTasks, nextPageToken, nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				initialNextPageToken := []byte{}
				_, err := store.GetReplicationTasksFromDLQ(ctx, &persistence.GetReplicationTasksFromDLQRequest{
					SourceClusterName: "sourceCluster",
					GetReplicationTasksRequest: persistence.GetReplicationTasksRequest{
						BatchSize:     10,
						NextPageToken: initialNextPageToken,
						ReadLevel:     0,
						MaxReadLevel:  100,
					},
				})

				return err
			},
			expectedError: nil,
		},
		{
			name: "GetReplicationTasksFromDLQ failure - invalid read levels",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				return newTestNosqlExecutionStore(nosqlplugin.NewMockDB(ctrl), log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.GetReplicationTasksFromDLQ(ctx, &persistence.GetReplicationTasksFromDLQRequest{
					SourceClusterName: "sourceCluster",
					GetReplicationTasksRequest: persistence.GetReplicationTasksRequest{
						ReadLevel:     100,
						MaxReadLevel:  50,
						BatchSize:     10,
						NextPageToken: []byte{},
					},
				})
				return err
			},
			expectedError: &types.BadRequestError{Message: "ReadLevel cannot be higher than MaxReadLevel"},
		},
		{
			name: "GetReplicationDLQSize success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					SelectReplicationDLQTasksCount(ctx, shardID, "sourceCluster").
					Return(int64(42), nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				resp, err := store.GetReplicationDLQSize(ctx, &persistence.GetReplicationDLQSizeRequest{
					SourceClusterName: "sourceCluster",
				})
				if err != nil {
					return err
				}
				if resp.Size != 42 {
					return errors.New("unexpected DLQ size")
				}
				return nil
			},
			expectedError: nil,
		},
		{
			name: "GetReplicationDLQSize failure - invalid source cluster name",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)
				mockDB.EXPECT().
					SelectReplicationDLQTasksCount(ctx, shardID, "").
					Return(int64(0), nil)
				return newTestNosqlExecutionStore(mockDB, log.NewNoop())
			},
			testFunc: func(store *nosqlExecutionStore) error {
				_, err := store.GetReplicationDLQSize(ctx, &persistence.GetReplicationDLQSizeRequest{
					SourceClusterName: "",
				})
				return err
			},
			expectedError: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			store := tc.setupMock(ctrl)
			err := tc.testFunc(store)

			if tc.expectedError != nil {
				var expectedErrType error
				require.ErrorAs(t, err, &expectedErrType, "Expected error type does not match.")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDeleteReplicationTaskFromDLQ(t *testing.T) {
	ctx := context.Background()
	shardID := 1

	tests := []struct {
		name          string
		sourceCluster string
		taskID        int64
		setupMock     func(*nosqlplugin.MockDB)
		expectedError error
	}{
		{
			name:          "success",
			sourceCluster: "sourceCluster",
			taskID:        1,
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().
					DeleteReplicationDLQTask(ctx, shardID, "sourceCluster", int64(1)).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name:          "database error",
			sourceCluster: "sourceCluster",
			taskID:        1,
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				mockDB.EXPECT().
					DeleteReplicationDLQTask(ctx, shardID, "sourceCluster", int64(1)).
					Return(errors.New("database error"))
			},
			expectedError: &types.InternalServiceError{Message: "database error"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockDB := nosqlplugin.NewMockDB(controller)
			store := newTestNosqlExecutionStore(mockDB, log.NewNoop())

			tc.setupMock(mockDB)

			err := store.DeleteReplicationTaskFromDLQ(ctx, &persistence.DeleteReplicationTaskFromDLQRequest{
				SourceClusterName: tc.sourceCluster,
				TaskID:            tc.taskID,
			})

			if tc.expectedError != nil {
				require.ErrorAs(t, err, &tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRangeDeleteReplicationTaskFromDLQ(t *testing.T) {
	ctx := context.Background()
	shardID := 1

	tests := []struct {
		name             string
		sourceCluster    string
		inclusiveBeginID int64
		exclusiveEndID   int64
		setupMock        func(*nosqlplugin.MockDB)
		expectedError    error
	}{
		{
			name:             "success",
			sourceCluster:    "sourceCluster",
			inclusiveBeginID: 1,
			exclusiveEndID:   100,
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().
					RangeDeleteReplicationDLQTasks(ctx, shardID, "sourceCluster", int64(1), int64(100)).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name:             "database error",
			sourceCluster:    "sourceCluster",
			inclusiveBeginID: 1,
			exclusiveEndID:   100,
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				mockDB.EXPECT().
					RangeDeleteReplicationDLQTasks(ctx, shardID, "sourceCluster", int64(1), int64(100)).
					Return(errors.New("database error"))
			},
			expectedError: &types.InternalServiceError{Message: "database error"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockDB := nosqlplugin.NewMockDB(controller)
			store := newTestNosqlExecutionStore(mockDB, log.NewNoop())

			tc.setupMock(mockDB)

			_, err := store.RangeDeleteReplicationTaskFromDLQ(ctx, &persistence.RangeDeleteReplicationTaskFromDLQRequest{
				SourceClusterName:    tc.sourceCluster,
				InclusiveBeginTaskID: tc.inclusiveBeginID,
				ExclusiveEndTaskID:   tc.exclusiveEndID,
			})

			if tc.expectedError != nil {
				require.ErrorAs(t, err, &tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateFailoverMarkerTasks(t *testing.T) {
	ctx := context.Background()
	shardID := 1

	tests := []struct {
		name          string
		rangeID       int64
		markers       []*persistence.FailoverMarkerTask
		setupMock     func(*nosqlplugin.MockDB, *serialization.MockTaskSerializer)
		expectedError error
	}{
		{
			name:    "success",
			rangeID: 123,
			markers: []*persistence.FailoverMarkerTask{
				{
					TaskData: persistence.TaskData{},
					DomainID: "testDomainID",
				},
			},
			setupMock: func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryReplication, gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte("1"),
					Encoding: common.EncodingTypeThriftRW,
				}, nil)
				mockDB.EXPECT().
					InsertReplicationTask(ctx, gomock.Any(), nosqlplugin.ShardCondition{ShardID: shardID, RangeID: 123}).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name:    "serialization error",
			rangeID: 123,
			markers: []*persistence.FailoverMarkerTask{
				{
					TaskData: persistence.TaskData{},
					DomainID: "testDomainID",
				},
			},
			setupMock: func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryReplication, gomock.Any()).Return(persistence.DataBlob{}, errors.New("some error"))
			},
			expectedError: errors.New("some error"),
		},
		{
			name:    "CreateFailoverMarkerTasks failure - ShardOperationConditionFailure",
			rangeID: 123,
			markers: []*persistence.FailoverMarkerTask{
				{
					TaskData: persistence.TaskData{},
					DomainID: "testDomainID",
				},
			},
			setupMock: func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryReplication, gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte("1"),
					Encoding: common.EncodingTypeThriftRW,
				}, nil)
				conditionFailureErr := &nosqlplugin.ShardOperationConditionFailure{
					RangeID: 123,                      // Use direct int64 value
					Details: "Shard condition failed", // Use direct string value
				}
				mockDB.EXPECT().
					InsertReplicationTask(ctx, gomock.Any(), nosqlplugin.ShardCondition{ShardID: shardID, RangeID: 123}).
					Return(conditionFailureErr) // Simulate ShardOperationConditionFailure
			},
			expectedError: &persistence.ShardOwnershipLostError{
				ShardID: shardID,
				Msg:     "Failed to create workflow execution.  Request RangeID: 123, columns: (Shard condition failed)",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockDB := nosqlplugin.NewMockDB(controller)
			mockTaskSerializer := serialization.NewMockTaskSerializer(controller)
			store := newTestNosqlExecutionStoreWithTaskSerializer(mockDB, log.NewNoop(), mockTaskSerializer)

			tc.setupMock(mockDB, mockTaskSerializer)

			err := store.CreateFailoverMarkerTasks(ctx, &persistence.CreateFailoverMarkersRequest{
				RangeID: tc.rangeID,
				Markers: tc.markers,
			})

			if tc.expectedError != nil {
				require.ErrorAs(t, err, &tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIsWorkflowExecutionExists(t *testing.T) {
	ctx := context.Background()
	gomockController := gomock.NewController(t)

	mockDB := nosqlplugin.NewMockDB(gomockController)
	store := &nosqlExecutionStore{
		shardID:    1,
		nosqlStore: nosqlStore{db: mockDB},
	}

	domainID := "testDomainID"
	workflowID := "testWorkflowID"
	runID := "testRunID"

	tests := []struct {
		name           string
		setupMock      func()
		request        *persistence.IsWorkflowExecutionExistsRequest
		expectedExists bool
		expectedError  error
	}{
		{
			name: "Workflow Execution Exists",
			setupMock: func() {
				mockDB.EXPECT().IsWorkflowExecutionExists(ctx, store.shardID, domainID, workflowID, runID).Return(true, nil)
			},
			request: &persistence.IsWorkflowExecutionExistsRequest{
				DomainID:   domainID,
				WorkflowID: workflowID,
				RunID:      runID,
			},
			expectedExists: true,
			expectedError:  nil,
		},
		{
			name: "Workflow Execution Does Not Exist",
			setupMock: func() {
				mockDB.EXPECT().IsWorkflowExecutionExists(ctx, store.shardID, domainID, workflowID, runID).Return(false, nil)
			},
			request: &persistence.IsWorkflowExecutionExistsRequest{
				DomainID:   domainID,
				WorkflowID: workflowID,
				RunID:      runID,
			},
			expectedExists: false,
			expectedError:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			response, err := store.IsWorkflowExecutionExists(ctx, tc.request)

			if tc.expectedError != nil {
				require.Error(t, err)
				// Optionally, further validate the error type or message
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedExists, response.Exists)
			}
		})
	}
}

func TestConflictResolveWorkflowExecution(t *testing.T) {
	ctx := context.Background()
	gomockController := gomock.NewController(t)

	mockDB := nosqlplugin.NewMockDB(gomockController)
	mockTaskSerializer := serialization.NewMockTaskSerializer(gomockController)
	store, err := NewExecutionStore(1, mockDB, log.NewNoop(), mockTaskSerializer, &persistence.DynamicConfiguration{
		EnableHistoryTaskDualWriteMode: func(...dynamicconfig.FilterOption) bool { return false },
	})
	require.NoError(t, err)

	tests := []struct {
		name          string
		setupMocks    func()
		getRequest    func() *persistence.InternalConflictResolveWorkflowExecutionRequest
		expectedError error
	}{
		{
			name: "DB Error on Reset Execution Insertion",
			setupMocks: func() {
				mockDB.EXPECT().UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("DB error")).Times(1)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsTimeoutError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsThrottlingError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsDBUnavailableError(gomock.Any()).Return(false).AnyTimes()
			},
			getRequest: func() *persistence.InternalConflictResolveWorkflowExecutionRequest {
				return newConflictResolveRequest(persistence.ConflictResolveWorkflowModeUpdateCurrent)
			},
			expectedError: &types.InternalServiceError{Message: "DB error"},
		},
		{
			name: "Unknown Conflict Resolution Mode",
			setupMocks: func() {
			},
			getRequest: func() *persistence.InternalConflictResolveWorkflowExecutionRequest {
				req := newConflictResolveRequest(-1) // Intentionally using an invalid mode
				return req
			},
			expectedError: &types.InternalServiceError{Message: "ConflictResolveWorkflowExecution: unknown mode: -1"},
		},
		{
			name: "Error on Shard Condition Mismatch",
			setupMocks: func() {
				// Simulate shard condition mismatch error by returning a ShardOperationConditionFailure error from the mock
				conditionFailure := &nosqlplugin.ShardOperationConditionFailure{
					RangeID: 124, // Example mismatched RangeID to trigger the condition failure
				}
				mockDB.EXPECT().UpdateWorkflowExecutionWithTasks(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(conditionFailure).Times(1)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsTimeoutError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsThrottlingError(gomock.Any()).Return(false).AnyTimes()
				mockDB.EXPECT().IsDBUnavailableError(gomock.Any()).Return(false).AnyTimes() // Ensure this call returns the simulated condition failure once
			},
			getRequest: func() *persistence.InternalConflictResolveWorkflowExecutionRequest {
				req := newConflictResolveRequest(persistence.ConflictResolveWorkflowModeUpdateCurrent)
				req.RangeID = 123 // Intentionally set to simulate the condition leading to a shard condition mismatch.
				return req
			},
			expectedError: &types.InternalServiceError{Message: "Shard error"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMocks()

			req := tc.getRequest()
			err := store.ConflictResolveWorkflowExecution(ctx, req)

			if tc.expectedError != nil {
				require.Error(t, err)
				assert.IsType(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func newCreateWorkflowExecutionRequest() *persistence.InternalCreateWorkflowExecutionRequest {
	return &persistence.InternalCreateWorkflowExecutionRequest{
		RangeID:                  123,
		Mode:                     persistence.CreateWorkflowModeBrandNew,
		PreviousRunID:            "previous-run-id",
		PreviousLastWriteVersion: 456,
		NewWorkflowSnapshot:      getNewWorkflowSnapshot(),
	}
}

func newGetWorkflowExecutionRequest() *persistence.InternalGetWorkflowExecutionRequest {
	return &persistence.InternalGetWorkflowExecutionRequest{
		DomainID: constants.TestDomainID,
		Execution: types.WorkflowExecution{
			WorkflowID: constants.TestWorkflowID,
			RunID:      constants.TestRunID,
		},
	}
}

func newUpdateWorkflowExecutionRequest() *persistence.InternalUpdateWorkflowExecutionRequest {
	return &persistence.InternalUpdateWorkflowExecutionRequest{
		RangeID: 123,
		UpdateWorkflowMutation: persistence.InternalWorkflowMutation{
			ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
				DomainID:    constants.TestDomainID,
				WorkflowID:  constants.TestWorkflowID,
				RunID:       constants.TestRunID,
				State:       persistence.WorkflowStateCreated,
				CloseStatus: persistence.WorkflowCloseStatusNone,
			},
			WorkflowRequests: []*persistence.WorkflowRequest{
				{
					RequestID:   constants.TestRequestID,
					Version:     1,
					RequestType: persistence.WorkflowRequestTypeStart,
				},
			},
		},
	}
}

func getNewWorkflowSnapshot() persistence.InternalWorkflowSnapshot {
	return persistence.InternalWorkflowSnapshot{
		VersionHistories: &persistence.DataBlob{},
		ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
			DomainID:   constants.TestDomainID,
			WorkflowID: constants.TestWorkflowID,
			RunID:      constants.TestRunID,
		},
		WorkflowRequests: []*persistence.WorkflowRequest{
			{
				RequestID:   constants.TestRequestID,
				Version:     1,
				RequestType: persistence.WorkflowRequestTypeStart,
			},
		},
	}
}
func newTestNosqlExecutionStore(db nosqlplugin.DB, logger log.Logger) *nosqlExecutionStore {
	return &nosqlExecutionStore{
		shardID:    1,
		nosqlStore: nosqlStore{logger: logger, db: db},
	}
}

func newInternalReplicationTaskInfo() persistence.InternalReplicationTaskInfo {
	var fixedCreationTime = time.Date(2024, time.April, 3, 14, 35, 44, 0, time.UTC)
	return persistence.InternalReplicationTaskInfo{
		DomainID:          "testDomainID",
		WorkflowID:        "testWorkflowID",
		RunID:             "testRunID",
		TaskID:            123,
		TaskType:          persistence.ReplicationTaskTypeHistory,
		FirstEventID:      1,
		NextEventID:       2,
		Version:           1,
		ScheduledID:       3,
		BranchToken:       []byte("branchToken"),
		NewRunBranchToken: []byte("newRunBranchToken"),
		CreationTime:      fixedCreationTime,
	}
}

func newConflictResolveRequest(mode persistence.ConflictResolveWorkflowMode) *persistence.InternalConflictResolveWorkflowExecutionRequest {
	return &persistence.InternalConflictResolveWorkflowExecutionRequest{
		Mode:    mode,
		RangeID: 123,
		CurrentWorkflowMutation: &persistence.InternalWorkflowMutation{
			ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
				DomainID:    "testDomainID",
				WorkflowID:  "testWorkflowID",
				RunID:       "currentRunID",
				State:       persistence.WorkflowStateCompleted,
				CloseStatus: persistence.WorkflowCloseStatusCompleted,
			},
		},
		ResetWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
			ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
				DomainID:    "testDomainID",
				WorkflowID:  "testWorkflowID",
				RunID:       "resetRunID",
				State:       persistence.WorkflowStateRunning,
				CloseStatus: persistence.WorkflowCloseStatusNone,
			},
		},
		NewWorkflowSnapshot: nil,
	}
}

func TestRangeCompleteHistoryTask(t *testing.T) {
	ctx := context.Background()
	shardID := 1

	tests := []struct {
		name          string
		request       *persistence.RangeCompleteHistoryTaskRequest
		setupMock     func(*nosqlplugin.MockDB)
		expectedError error
	}{
		{
			name: "success - scheduled timer task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTimer,
				InclusiveMinTaskKey: persistence.HistoryTaskKey{ScheduledTime: time.Unix(0, 0)},
				ExclusiveMaxTaskKey: persistence.HistoryTaskKey{ScheduledTime: time.Unix(0, 0).Add(time.Minute)},
			},
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteTimerTasks(ctx, shardID, time.Unix(0, 0), time.Unix(0, 0).Add(time.Minute)).Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "success - immediate transfer task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTransfer,
				InclusiveMinTaskKey: persistence.HistoryTaskKey{TaskID: 100},
				ExclusiveMaxTaskKey: persistence.HistoryTaskKey{TaskID: 200},
			},
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteTransferTasks(ctx, shardID, int64(100), int64(200)).Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "success - immediate replication task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryReplication,
				InclusiveMinTaskKey: persistence.HistoryTaskKey{TaskID: 100}, // this is ignored by replication task
				ExclusiveMaxTaskKey: persistence.HistoryTaskKey{TaskID: 200},
			},
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteReplicationTasks(ctx, shardID, int64(200)).Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "unknown task category error",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategory{},
			},
			setupMock:     func(mockDB *nosqlplugin.MockDB) {},
			expectedError: &types.BadRequestError{},
		},
		{
			name: "database error on timer task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTimer,
				InclusiveMinTaskKey: persistence.HistoryTaskKey{ScheduledTime: time.Unix(0, 0)},
				ExclusiveMaxTaskKey: persistence.HistoryTaskKey{ScheduledTime: time.Unix(0, 0).Add(time.Minute)},
			},
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteTimerTasks(ctx, shardID, time.Unix(0, 0), time.Unix(0, 0).Add(time.Minute)).Return(errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "database error on transfer task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTransfer,
				InclusiveMinTaskKey: persistence.HistoryTaskKey{TaskID: 100},
				ExclusiveMaxTaskKey: persistence.HistoryTaskKey{TaskID: 200},
			},
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteTransferTasks(ctx, shardID, int64(100), int64(200)).Return(errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "database error on replication task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryReplication,
				InclusiveMinTaskKey: persistence.HistoryTaskKey{TaskID: 100},
				ExclusiveMaxTaskKey: persistence.HistoryTaskKey{TaskID: 200},
			},
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteReplicationTasks(ctx, shardID, int64(200)).Return(errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			defer controller.Finish()

			mockDB := nosqlplugin.NewMockDB(controller)
			store := &nosqlExecutionStore{nosqlStore: nosqlStore{db: mockDB}, shardID: shardID}

			tc.setupMock(mockDB)

			_, err := store.RangeCompleteHistoryTask(ctx, tc.request)
			if tc.expectedError != nil {
				require.ErrorAs(t, err, &tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
