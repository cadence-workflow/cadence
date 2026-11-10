package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/batcher"
)

const batcherDomainID = "batcher-local-domain-id"

func TestStartBatchOperation(t *testing.T) {
	validRequest := &types.StartBatchOperationRequest{
		Domain:  testDomain,
		BatchID: "my-batch-id",
		Query:   "WorkflowType = 'my-workflow-type'",
		Operation: &types.BatchOperation{
			OperationType: types.BatchOperationTypeSignal.Ptr(),
			SignalBatchOperation: &types.SignalBatchOperation{
				SignalName: "my-signal",
				Input:      []byte("signal-input"),
			},
		},
		Reason: "my-reason",
	}

	tests := map[string]struct {
		request *types.StartBatchOperationRequest
		mockFn  func(*scheduleTestFixture)
		wantErr bool
	}{
		"nil request": {
			request: nil,
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty domain": {
			request: &types.StartBatchOperationRequest{},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty batch ID": {
			request: &types.StartBatchOperationRequest{Domain: testDomain},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty query": {
			request: &types.StartBatchOperationRequest{Domain: testDomain, BatchID: "b1"},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"nil operation": {
			request: &types.StartBatchOperationRequest{Domain: testDomain, BatchID: "b1", Query: "q"},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"invalid operation type": {
			request: &types.StartBatchOperationRequest{
				Domain:    testDomain,
				BatchID:   "b1",
				Query:     "q",
				Operation: &types.BatchOperation{},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"missing reason": {
			request: &types.StartBatchOperationRequest{
				Domain:  testDomain,
				BatchID: "b1",
				Query:   "q",
				Operation: &types.BatchOperation{
					OperationType: types.BatchOperationTypeTerminate.Ptr(),
				},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"signal missing signal name": {
			request: &types.StartBatchOperationRequest{
				Domain:  testDomain,
				BatchID: "b1",
				Query:   "q",
				Reason:  "my-reason",
				Operation: &types.BatchOperation{
					OperationType:        types.BatchOperationTypeSignal.Ptr(),
					SignalBatchOperation: &types.SignalBatchOperation{},
				},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"replicate missing clusters": {
			request: &types.StartBatchOperationRequest{
				Domain:  testDomain,
				BatchID: "b1",
				Query:   "q",
				Reason:  "my-reason",
				Operation: &types.BatchOperation{
					OperationType:           types.BatchOperationTypeReplicate.Ptr(),
					ReplicateBatchOperation: &types.ReplicateBatchOperation{},
				},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"domain not found": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return("", errors.New("not found")).AnyTimes()
			},
			wantErr: true,
		},
		"history error": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("internal error"))
			},
			wantErr: true,
		},
		"already started is treated as idempotent success": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, &types.WorkflowExecutionAlreadyStartedError{Message: "already started"})
			},
			wantErr: false,
		},
		"signal success": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistoryStartWorkflowExecutionRequest, _ ...yarpc.CallOption) (*types.StartWorkflowExecutionResponse, error) {
						assert.Equal(t, batcherDomainID, req.DomainUUID)
						assert.Equal(t, "cadence-sys-batch-operation:test-domain:my-batch-id", req.StartRequest.WorkflowID)
						assert.Equal(t, constants.BatcherLocalDomainName, req.StartRequest.Domain)
						assert.Equal(t, batcher.BatchWFV2TypeName, req.StartRequest.WorkflowType.Name)
						assert.Equal(t, batcher.BatcherTaskListName, req.StartRequest.TaskList.Name)
						require.NotNil(t, req.StartRequest.RetryPolicy)

						var reason string
						require.NoError(t, json.Unmarshal(req.StartRequest.Memo.Fields["Reason"], &reason))
						assert.Equal(t, "my-reason", reason)

						var customDomain string
						require.NoError(t, json.Unmarshal(req.StartRequest.SearchAttributes.IndexedFields["CustomDomain"], &customDomain))
						assert.Equal(t, testDomain, customDomain)

						var params batcher.BatchParams
						require.NoError(t, json.Unmarshal(req.StartRequest.Input, &params))
						assert.Equal(t, testDomain, params.DomainName)
						assert.Equal(t, "WorkflowType = 'my-workflow-type'", params.Query)
						assert.Equal(t, batcher.BatchTypeSignal, params.BatchType)
						assert.Equal(t, "my-signal", params.SignalParams.SignalName)
						require.NotNil(t, params.Operation)
						assert.Equal(t, types.BatchOperationTypeSignal, params.Operation.GetOperationType())
						assert.Equal(t, "my-signal", params.Operation.GetSignalBatchOperation().GetSignalName())

						return &types.StartWorkflowExecutionResponse{RunID: "test-run-id"}, nil
					})
			},
			wantErr: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newScheduleTestFixture(t)
			defer f.finish()
			tt.mockFn(f)

			resp, err := f.handler.StartBatchOperation(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, "my-batch-id", resp.BatchID)
			}
		})
	}
}

func TestStopBatchOperation(t *testing.T) {
	validRequest := &types.StopBatchOperationRequest{
		Domain:  testDomain,
		BatchID: "my-batch-id",
		Reason:  "my-reason",
	}

	tests := map[string]struct {
		request *types.StopBatchOperationRequest
		mockFn  func(*scheduleTestFixture)
		wantErr bool
	}{
		"nil request": {
			request: nil,
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty domain": {
			request: &types.StopBatchOperationRequest{},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty batch ID": {
			request: &types.StopBatchOperationRequest{Domain: testDomain},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"domain not found": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return("", errors.New("not found")).AnyTimes()
			},
			wantErr: true,
		},
		"history error": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(errors.New("internal error"))
			},
			wantErr: true,
		},
		"success": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistoryTerminateWorkflowExecutionRequest, _ ...yarpc.CallOption) error {
						assert.Equal(t, batcherDomainID, req.DomainUUID)
						assert.Equal(t, constants.BatcherLocalDomainName, req.TerminateRequest.Domain)
						assert.Equal(t, "cadence-sys-batch-operation:test-domain:my-batch-id", req.TerminateRequest.WorkflowExecution.WorkflowID)
						assert.Equal(t, "my-reason", req.TerminateRequest.Reason)
						return nil
					})
			},
			wantErr: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newScheduleTestFixture(t)
			defer f.finish()
			tt.mockFn(f)

			resp, err := f.handler.StopBatchOperation(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, resp)
			}
		})
	}
}
