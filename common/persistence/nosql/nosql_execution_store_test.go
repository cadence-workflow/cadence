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
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
		expectedError error
	}{
		{
			name: "success",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				mockDB.EXPECT().
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), nil, gomock.Any(), gomock.Any()).
					Return(nil)
			},
			request:       newUpdateWorkflowExecutionRequest,
			expectedError: nil,
		},
		{
			name: "Success - UpdateWorkflowModeIgnoreCurrent",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				mockDB.EXPECT().
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), nil, gomock.Any(), gomock.Any()).
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
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), nil, gomock.Any(), gomock.Any()).
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
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), nil, gomock.Any(), gomock.Any()).
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
		{
			name: "Update with new snapshot (continue as new)",
			setupMock: func(mockDB *nosqlplugin.MockDB, shardID int) {
				// validate the NewWorkflowSnapshot's info is used to create the active cluster selection policy row
				info := newUpdateWorkflowExecutionRequestForContinueAsNew().NewWorkflowSnapshot.ExecutionInfo
				plcy := &nosqlplugin.ActiveClusterSelectionPolicyRow{
					ShardID:    shardID,
					DomainID:   info.DomainID,
					WorkflowID: info.WorkflowID,
					RunID:      info.RunID,
					Policy:     info.ActiveClusterSelectionPolicy,
				}
				mockDB.EXPECT().
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Not(nil), plcy, nil, gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
			},
			request: func() *persistence.InternalUpdateWorkflowExecutionRequest {
				req := newUpdateWorkflowExecutionRequestForContinueAsNew()
				return req
			},
			expectedError: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			mockDB := nosqlplugin.NewMockDB(controller)
			mockTaskSerializer := serialization.NewMockTaskSerializer(controller)
			store, _ := NewExecutionStore(1, mockDB, log.NewNoop(), mockTaskSerializer, &persistence.DynamicConfiguration{
				EnableHistoryTaskDualWriteMode: func(...dynamicproperties.FilterOption) bool { return false },
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					InsertWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Nil(), gomock.Any(), gomock.Any()).
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
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Nil(), gomock.Any(), gomock.Any()).
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
					UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Nil(), gomock.Any(), gomock.Any()).
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
			name: "GetReplicationTasksFromDLQ success",
			setupMock: func(ctrl *gomock.Controller) *nosqlExecutionStore {
				mockDB := nosqlplugin.NewMockDB(ctrl)

				nextPageToken := []byte("next-page-token")
				replicationTasks := []*nosqlplugin.HistoryMigrationTask{}
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
					BatchSize:         10,
					NextPageToken:     initialNextPageToken,
					ReadLevel:         0,
					MaxReadLevel:      100,
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
					ReadLevel:         100,
					MaxReadLevel:      50,
					BatchSize:         10,
					NextPageToken:     []byte{},
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
					Encoding: commonconstants.EncodingTypeThriftRW,
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
					Encoding: commonconstants.EncodingTypeThriftRW,
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
		EnableHistoryTaskDualWriteMode: func(...dynamicproperties.FilterOption) bool { return false },
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
				mockDB.EXPECT().UpdateWorkflowExecutionWithTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("DB error")).Times(1)
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
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
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

func newUpdateWorkflowExecutionRequestForContinueAsNew() *persistence.InternalUpdateWorkflowExecutionRequest {
	return &persistence.InternalUpdateWorkflowExecutionRequest{
		RangeID: 123,
		Mode:    persistence.UpdateWorkflowModeUpdateCurrent,
		UpdateWorkflowMutation: persistence.InternalWorkflowMutation{
			ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
				DomainID:    constants.TestDomainID,
				WorkflowID:  constants.TestWorkflowID,
				RunID:       constants.TestRunID,
				State:       persistence.WorkflowStateCompleted,
				CloseStatus: persistence.WorkflowCloseStatusContinuedAsNew,
			},
			WorkflowRequests: []*persistence.WorkflowRequest{
				{
					RequestID:   constants.TestRequestID,
					Version:     1,
					RequestType: persistence.WorkflowRequestTypeStart,
				},
			},
		},
		NewWorkflowSnapshot: &persistence.InternalWorkflowSnapshot{
			VersionHistories: &persistence.DataBlob{},
			ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
				DomainID:   constants.TestDomainID,
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
				ActiveClusterSelectionPolicy: &persistence.DataBlob{
					Encoding: "json",
					Data:     []byte(`{"policy": "abc"}`),
				},
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
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 0),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0).Add(time.Minute), 0),
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
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
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
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100), // this is ignored by replication task
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
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
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 0),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0).Add(time.Minute), 0),
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
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
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
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
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

func TestGetHistoryTasks(t *testing.T) {
	ctx := context.Background()
	shardID := 1

	tests := []struct {
		name                             string
		request                          *persistence.GetHistoryTasksRequest
		readNoSQLHistoryTaskFromDataBlob bool
		setupMock                        func(*nosqlplugin.MockDB, *serialization.MockTaskSerializer)
		expectedError                    error
		expectedTasks                    []persistence.Task
	}{
		{
			name: "success - get immediate transfer tasks",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTransfer,
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
				PageSize:            10,
				NextPageToken:       []byte("next-page-token"),
			},
			setupMock: func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectTransferTasksOrderByTaskID(ctx, shardID, 10, []byte("next-page-token"), int64(100), int64(200)).Return([]*nosqlplugin.HistoryMigrationTask{
					{
						Transfer: &persistence.TransferTaskInfo{
							DomainID:            "testDomainID",
							WorkflowID:          "testWorkflowID",
							RunID:               "testRunID",
							VisibilityTimestamp: time.Unix(0, 0),
							TaskID:              1,
							TaskType:            persistence.TransferTaskTypeDecisionTask,
							ScheduleID:          1,
							Version:             1,
							TaskList:            "testTaskList",
						},
						Task: &persistence.DataBlob{Data: []byte("task"), Encoding: "json"},
					},
				}, nil, nil)
			},
			expectedError: nil,
			expectedTasks: []persistence.Task{
				&persistence.DecisionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "testDomainID",
						WorkflowID: "testWorkflowID",
						RunID:      "testRunID",
					},
					TaskData: persistence.TaskData{
						TaskID:              1,
						Version:             1,
						VisibilityTimestamp: time.Unix(0, 0),
					},
					ScheduleID: 1,
					TaskList:   "testTaskList",
				},
			},
		},
		{
			name: "success - get immediate transfer tasks from data blob",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTransfer,
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
				PageSize:            10,
				NextPageToken:       []byte("next-page-token"),
			},
			readNoSQLHistoryTaskFromDataBlob: true,
			setupMock: func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectTransferTasksOrderByTaskID(ctx, shardID, 10, []byte("next-page-token"), int64(100), int64(200)).Return([]*nosqlplugin.HistoryMigrationTask{
					{
						Task:   &persistence.DataBlob{Data: []byte("task"), Encoding: "json"},
						TaskID: 1,
					},
				}, nil, nil)
				mockTaskSerializer.EXPECT().DeserializeTask(persistence.HistoryTaskCategoryTransfer, gomock.Any()).Return(&persistence.DecisionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "testDomainID",
						WorkflowID: "testWorkflowID",
						RunID:      "testRunID",
					},
					TaskData: persistence.TaskData{
						TaskID:              1,
						Version:             1,
						VisibilityTimestamp: time.Unix(0, 0),
					},
					ScheduleID: 1,
					TaskList:   "testTaskList",
				}, nil)
			},
			expectedError: nil,
			expectedTasks: []persistence.Task{
				&persistence.DecisionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "testDomainID",
						WorkflowID: "testWorkflowID",
						RunID:      "testRunID",
					},
					TaskData: persistence.TaskData{
						TaskID:              1,
						Version:             1,
						VisibilityTimestamp: time.Unix(0, 0),
					},
					ScheduleID: 1,
					TaskList:   "testTaskList",
				},
			},
		},
		{
			name: "success - get scheduled timer tasks",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTimer,
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 0),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0).Add(time.Minute), 0),
				PageSize:            10,
				NextPageToken:       []byte("next-page-token"),
			},
			setupMock: func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectTimerTasksOrderByVisibilityTime(ctx, shardID, 10, []byte("next-page-token"), time.Unix(0, 0), time.Unix(0, 0).Add(time.Minute)).Return([]*nosqlplugin.HistoryMigrationTask{
					{
						Timer: &persistence.TimerTaskInfo{
							DomainID:            "testDomainID",
							WorkflowID:          "testWorkflowID",
							RunID:               "testRunID",
							VisibilityTimestamp: time.Unix(0, 0),
							TaskID:              1,
							TaskType:            persistence.TaskTypeUserTimer,
							Version:             1,
							EventID:             12,
						},
						Task: &persistence.DataBlob{Data: []byte("task"), Encoding: "json"},
					},
				}, nil, nil)
			},
			expectedError: nil,
			expectedTasks: []persistence.Task{
				&persistence.UserTimerTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "testDomainID",
						WorkflowID: "testWorkflowID",
						RunID:      "testRunID",
					},
					TaskData: persistence.TaskData{
						TaskID:              1,
						Version:             1,
						VisibilityTimestamp: time.Unix(0, 0),
					},
					EventID: 12,
				},
			},
		},
		{
			name: "success - get scheduled timer tasks from data blob",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTimer,
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 0),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0).Add(time.Minute), 0),
				PageSize:            10,
				NextPageToken:       []byte("next-page-token"),
			},
			readNoSQLHistoryTaskFromDataBlob: true,
			setupMock: func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectTimerTasksOrderByVisibilityTime(ctx, shardID, 10, []byte("next-page-token"), time.Unix(0, 0), time.Unix(0, 0).Add(time.Minute)).Return([]*nosqlplugin.HistoryMigrationTask{
					{
						Task:          &persistence.DataBlob{Data: []byte("task"), Encoding: "json"},
						TaskID:        1,
						ScheduledTime: time.Unix(1, 1),
					},
				}, nil, nil)
				mockTaskSerializer.EXPECT().DeserializeTask(persistence.HistoryTaskCategoryTimer, gomock.Any()).Return(&persistence.UserTimerTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "testDomainID",
						WorkflowID: "testWorkflowID",
						RunID:      "testRunID",
					},
					TaskData: persistence.TaskData{
						Version: 1,
					},
					EventID: 12,
				}, nil)
			},
			expectedError: nil,
			expectedTasks: []persistence.Task{
				&persistence.UserTimerTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "testDomainID",
						WorkflowID: "testWorkflowID",
						RunID:      "testRunID",
					},
					TaskData: persistence.TaskData{
						TaskID:              1,
						Version:             1,
						VisibilityTimestamp: time.Unix(1, 1),
					},
					EventID: 12,
				},
			},
		},
		{
			name: "success - get immediate replication tasks",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory:        persistence.HistoryTaskCategoryReplication,
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
				PageSize:            10,
				NextPageToken:       []byte("next-page-token"),
			},
			setupMock: func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectReplicationTasksOrderByTaskID(ctx, shardID, 10, []byte("next-page-token"), int64(100), int64(200)).Return([]*nosqlplugin.HistoryMigrationTask{
					{
						Replication: &persistence.InternalReplicationTaskInfo{
							DomainID:          "testDomainID",
							WorkflowID:        "testWorkflowID",
							RunID:             "testRunID",
							TaskID:            1,
							TaskType:          persistence.ReplicationTaskTypeHistory,
							Version:           1,
							ScheduledID:       3,
							FirstEventID:      1,
							NextEventID:       2,
							BranchToken:       []byte("branchToken"),
							NewRunBranchToken: []byte("newRunBranchToken"),
							CreationTime:      time.Unix(0, 0),
						},
						Task: &persistence.DataBlob{Data: []byte("task"), Encoding: "json"},
					},
				}, nil, nil)
			},
			expectedError: nil,
			expectedTasks: []persistence.Task{
				&persistence.HistoryReplicationTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "testDomainID",
						WorkflowID: "testWorkflowID",
						RunID:      "testRunID",
					},
					TaskData: persistence.TaskData{
						TaskID:              1,
						Version:             1,
						VisibilityTimestamp: time.Unix(0, 0),
					},
					FirstEventID:      1,
					NextEventID:       2,
					BranchToken:       []byte("branchToken"),
					NewRunBranchToken: []byte("newRunBranchToken"),
				},
			},
		},
		{
			name: "success - get immediate replication tasks from data blob",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory:        persistence.HistoryTaskCategoryReplication,
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
				PageSize:            10,
				NextPageToken:       []byte("next-page-token"),
			},
			readNoSQLHistoryTaskFromDataBlob: true,
			setupMock: func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectReplicationTasksOrderByTaskID(ctx, shardID, 10, []byte("next-page-token"), int64(100), int64(200)).Return([]*nosqlplugin.HistoryMigrationTask{
					{
						Task:   &persistence.DataBlob{Data: []byte("task"), Encoding: "json"},
						TaskID: 1,
					},
				}, nil, nil)
				mockTaskSerializer.EXPECT().DeserializeTask(persistence.HistoryTaskCategoryReplication, gomock.Any()).Return(&persistence.HistoryReplicationTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "testDomainID",
						WorkflowID: "testWorkflowID",
						RunID:      "testRunID",
					},
					TaskData: persistence.TaskData{
						Version:             1,
						VisibilityTimestamp: time.Unix(0, 0),
					},
					FirstEventID:      1,
					NextEventID:       2,
					BranchToken:       []byte("branchToken"),
					NewRunBranchToken: []byte("newRunBranchToken"),
				}, nil)
			},
			expectedError: nil,
			expectedTasks: []persistence.Task{
				&persistence.HistoryReplicationTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "testDomainID",
						WorkflowID: "testWorkflowID",
						RunID:      "testRunID",
					},
					TaskData: persistence.TaskData{
						TaskID:              1,
						Version:             1,
						VisibilityTimestamp: time.Unix(0, 0),
					},
					FirstEventID:      1,
					NextEventID:       2,
					BranchToken:       []byte("branchToken"),
					NewRunBranchToken: []byte("newRunBranchToken"),
				},
			},
		},
		{
			name: "database error on transfer task retrieval",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory: persistence.HistoryTaskCategoryTransfer,
				PageSize:     10,
			},
			setupMock: func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectTransferTasksOrderByTaskID(ctx, shardID, 10, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "database error on replication task retrieval",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory: persistence.HistoryTaskCategoryReplication,
				PageSize:     10,
			},
			setupMock: func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectReplicationTasksOrderByTaskID(ctx, shardID, 10, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "database error on timer task retrieval",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory: persistence.HistoryTaskCategoryTimer,
				PageSize:     10,
			},
			setupMock: func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectTimerTasksOrderByVisibilityTime(ctx, shardID, 10, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "unknown task category error",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory: persistence.HistoryTaskCategory{},
			},
			setupMock:     func(mockDB *nosqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {},
			expectedError: &types.BadRequestError{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			defer controller.Finish()

			mockDB := nosqlplugin.NewMockDB(controller)
			mockTaskSerializer := serialization.NewMockTaskSerializer(controller)
			store := &nosqlExecutionStore{nosqlStore: nosqlStore{db: mockDB, dc: &persistence.DynamicConfiguration{ReadNoSQLHistoryTaskFromDataBlob: func(...dynamicproperties.FilterOption) bool {
				return tc.readNoSQLHistoryTaskFromDataBlob
			}}}, shardID: shardID, taskSerializer: mockTaskSerializer}

			tc.setupMock(mockDB, mockTaskSerializer)

			resp, err := store.GetHistoryTasks(ctx, tc.request)
			if tc.expectedError != nil {
				require.ErrorAs(t, err, &tc.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedTasks, resp.Tasks)
			}
		})
	}
}

func TestCompleteHistoryTask(t *testing.T) {
	ctx := context.Background()
	shardID := 1

	tests := []struct {
		name          string
		request       *persistence.CompleteHistoryTaskRequest
		setupMock     func(*nosqlplugin.MockDB)
		expectedError error
	}{
		{
			name: "success - complete scheduled timer task",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategoryTimer,
				TaskKey:      persistence.NewHistoryTaskKey(time.Unix(10, 10), 1),
			},
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().DeleteTimerTask(ctx, shardID, int64(1), time.Unix(10, 10)).Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "success - complete immediate transfer task",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategoryTransfer,
				TaskKey:      persistence.NewImmediateTaskKey(2),
			},
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().DeleteTransferTask(ctx, shardID, int64(2)).Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "success - complete immediate replication task",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategoryReplication,
				TaskKey:      persistence.NewImmediateTaskKey(3),
			},
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().DeleteReplicationTask(ctx, shardID, int64(3)).Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "unknown task category type",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategory{},
			},
			setupMock:     func(mockDB *nosqlplugin.MockDB) {},
			expectedError: &types.BadRequestError{},
		},
		{
			name: "delete timer task error",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategoryTimer,
				TaskKey:      persistence.NewHistoryTaskKey(time.Unix(10, 10), 1),
			},
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().DeleteTimerTask(ctx, shardID, int64(1), time.Unix(10, 10)).Return(errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "delete transfer task error",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategoryTransfer,
				TaskKey:      persistence.NewImmediateTaskKey(2),
			},
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().DeleteTransferTask(ctx, shardID, int64(2)).Return(errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "delete replication task error",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategoryReplication,
				TaskKey:      persistence.NewImmediateTaskKey(3),
			},
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().DeleteReplicationTask(ctx, shardID, int64(3)).Return(errors.New("db error"))
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
			store := &nosqlExecutionStore{nosqlStore: nosqlStore{db: mockDB, dc: &persistence.DynamicConfiguration{}}, shardID: shardID}

			tc.setupMock(mockDB)

			err := store.CompleteHistoryTask(ctx, tc.request)
			if tc.expectedError != nil {
				require.ErrorAs(t, err, &tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
