package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
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

func TestDescribeBatchOperation(t *testing.T) {
	validRequest := &types.DescribeBatchOperationRequest{
		Domain:  testDomain,
		BatchID: "my-batch-id",
	}
	reasonBytes, err := json.Marshal("my-reason")
	require.NoError(t, err)
	operatorBytes, err := json.Marshal("my-operator")
	require.NoError(t, err)

	tests := map[string]struct {
		request  *types.DescribeBatchOperationRequest
		mockFn   func(*scheduleTestFixture)
		wantErr  bool
		validate func(*testing.T, *types.DescribeBatchOperationResponse)
	}{
		"nil request": {
			request: nil,
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty domain": {
			request: &types.DescribeBatchOperationRequest{},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty batch ID": {
			request: &types.DescribeBatchOperationRequest{Domain: testDomain},
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
				f.historyClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("internal error"))
			},
			wantErr: true,
		},
		"running with progress": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				hbd, err := json.Marshal(batcher.HeartBeatDetails{TotalEstimate: 10, SuccessCount: 3, ErrorCount: 1, RPS: 50, Concurrency: 5})
				require.NoError(t, err)
				f.historyClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistoryDescribeWorkflowExecutionRequest, _ ...yarpc.CallOption) (*types.DescribeWorkflowExecutionResponse, error) {
						assert.Equal(t, batcherDomainID, req.DomainUUID)
						assert.Equal(t, "cadence-sys-batch-operation:test-domain:my-batch-id", req.Request.Execution.WorkflowID)
						return &types.DescribeWorkflowExecutionResponse{
							WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
								Memo:             &types.Memo{Fields: map[string][]byte{"Reason": reasonBytes}},
								SearchAttributes: &types.SearchAttributes{IndexedFields: map[string][]byte{"Operator": operatorBytes}},
							},
							PendingActivities: []*types.PendingActivityInfo{{HeartbeatDetails: hbd}},
						}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, resp *types.DescribeBatchOperationResponse) {
				assert.Equal(t, types.BatchOperationStateRunning, resp.State)
				assert.Equal(t, "my-reason", resp.Reason)
				assert.Equal(t, "my-operator", resp.StartedBy)
				require.NotNil(t, resp.Progress)
				assert.Equal(t, int64(10), resp.Progress.TotalEstimate)
				assert.Equal(t, int64(3), resp.Progress.SuccessCount)
				assert.Equal(t, int64(1), resp.Progress.ErrorCount)
				require.NotNil(t, resp.Limits)
				assert.Equal(t, int32(50), resp.Limits.RPS)
				assert.Equal(t, int32(5), resp.Limits.Concurrency)
				assert.Nil(t, resp.StartTime)
				assert.Nil(t, resp.CloseTime)
			},
		},
		"completed with no pending activity": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.DescribeWorkflowExecutionResponse{
						WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
							CloseStatus: types.WorkflowExecutionCloseStatusCompleted.Ptr(),
						},
					}, nil)
			},
			wantErr: false,
			validate: func(t *testing.T, resp *types.DescribeBatchOperationResponse) {
				assert.Equal(t, types.BatchOperationStateCompleted, resp.State)
				assert.Nil(t, resp.Progress)
				assert.Nil(t, resp.Limits)
			},
		},
		"terminated maps to stopped": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.DescribeWorkflowExecutionResponse{
						WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
							CloseStatus: types.WorkflowExecutionCloseStatusTerminated.Ptr(),
						},
					}, nil)
			},
			wantErr: false,
			validate: func(t *testing.T, resp *types.DescribeBatchOperationResponse) {
				assert.Equal(t, types.BatchOperationStateStopped, resp.State)
			},
		},
		"failed maps to failed": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.DescribeWorkflowExecutionResponse{
						WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
							CloseStatus: types.WorkflowExecutionCloseStatusFailed.Ptr(),
						},
					}, nil)
			},
			wantErr: false,
			validate: func(t *testing.T, resp *types.DescribeBatchOperationResponse) {
				assert.Equal(t, types.BatchOperationStateFailed, resp.State)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newScheduleTestFixture(t)
			defer f.finish()
			tt.mockFn(f)

			resp, err := f.handler.DescribeBatchOperation(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, resp)
				if tt.validate != nil {
					tt.validate(t, resp)
				}
			}
		})
	}
}

func TestUpdateBatchOperation(t *testing.T) {
	tests := map[string]struct {
		request *types.UpdateBatchOperationRequest
		mockFn  func(*scheduleTestFixture)
		wantErr bool
	}{
		"nil request": {
			request: nil,
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty domain": {
			request: &types.UpdateBatchOperationRequest{},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty batch ID": {
			request: &types.UpdateBatchOperationRequest{Domain: testDomain},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"negative RPS": {
			request: &types.UpdateBatchOperationRequest{Domain: testDomain, BatchID: "my-batch-id", RPS: -1},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"negative concurrency": {
			request: &types.UpdateBatchOperationRequest{Domain: testDomain, BatchID: "my-batch-id", Concurrency: -1},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"no fields set": {
			request: &types.UpdateBatchOperationRequest{Domain: testDomain, BatchID: "my-batch-id"},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"domain not found": {
			request: &types.UpdateBatchOperationRequest{Domain: testDomain, BatchID: "my-batch-id", RPS: 100},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return("", errors.New("not found")).AnyTimes()
			},
			wantErr: true,
		},
		"history error": {
			request: &types.UpdateBatchOperationRequest{Domain: testDomain, BatchID: "my-batch-id", RPS: 100},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(errors.New("internal error"))
			},
			wantErr: true,
		},
		"entity not exists is a real error": {
			request: &types.UpdateBatchOperationRequest{Domain: testDomain, BatchID: "my-batch-id", RPS: 100},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.EntityNotExistsError{Message: "not found"})
			},
			wantErr: true,
		},
		"success, RPS only": {
			request: &types.UpdateBatchOperationRequest{Domain: testDomain, BatchID: "my-batch-id", RPS: 100},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistorySignalWorkflowExecutionRequest, _ ...yarpc.CallOption) error {
						assert.Equal(t, batcherDomainID, req.DomainUUID)
						assert.Equal(t, constants.BatcherLocalDomainName, req.SignalRequest.Domain)
						assert.Equal(t, "cadence-sys-batch-operation:test-domain:my-batch-id", req.SignalRequest.WorkflowExecution.WorkflowID)
						assert.Equal(t, batcher.SignalNameTune, req.SignalRequest.SignalName)
						var sig batcher.TuneSignal
						require.NoError(t, json.Unmarshal(req.SignalRequest.Input, &sig))
						assert.Equal(t, 100, sig.RPS)
						assert.Equal(t, 0, sig.Concurrency)
						return nil
					})
			},
			wantErr: false,
		},
		"success, concurrency only": {
			request: &types.UpdateBatchOperationRequest{Domain: testDomain, BatchID: "my-batch-id", Concurrency: 10},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistorySignalWorkflowExecutionRequest, _ ...yarpc.CallOption) error {
						var sig batcher.TuneSignal
						require.NoError(t, json.Unmarshal(req.SignalRequest.Input, &sig))
						assert.Equal(t, 0, sig.RPS)
						assert.Equal(t, 10, sig.Concurrency)
						return nil
					})
			},
			wantErr: false,
		},
		"success, both set": {
			request: &types.UpdateBatchOperationRequest{Domain: testDomain, BatchID: "my-batch-id", RPS: 100, Concurrency: 10},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistorySignalWorkflowExecutionRequest, _ ...yarpc.CallOption) error {
						var sig batcher.TuneSignal
						require.NoError(t, json.Unmarshal(req.SignalRequest.Input, &sig))
						assert.Equal(t, 100, sig.RPS)
						assert.Equal(t, 10, sig.Concurrency)
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

			resp, err := f.handler.UpdateBatchOperation(context.Background(), tt.request)
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
				f.domainCache.EXPECT().GetDomainID(testDomain).Return("", errors.New("not found")).Times(1)
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
		"entity not exists is a real error": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.EntityNotExistsError{Message: "not found"})
			},
			wantErr: true,
		},
		"already completed is treated as idempotent success": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.WorkflowExecutionAlreadyCompletedError{Message: "already completed"})
			},
			wantErr: false,
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

func TestListBatchOperations(t *testing.T) {
	operatorBytes, err := json.Marshal("my-operator")
	require.NoError(t, err)

	tests := map[string]struct {
		request  *types.ListBatchOperationsRequest
		mockFn   func(*scheduleTestFixture)
		wantErr  bool
		validate func(*testing.T, *types.ListBatchOperationsResponse)
	}{
		"nil request": {
			request: nil,
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty domain": {
			request: &types.ListBatchOperationsRequest{},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"invalid state value": {
			request: &types.ListBatchOperationsRequest{Domain: testDomain, State: types.BatchOperationState(99)},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"domain not found": {
			request: &types.ListBatchOperationsRequest{Domain: testDomain},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return("", errors.New("not found")).AnyTimes()
			},
			wantErr: true,
		},
		"visibility error": {
			request: &types.ListBatchOperationsRequest{Domain: testDomain},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.mockResource.VisibilityMgr.On("ListWorkflowExecutions", mock.Anything, mock.Anything).
					Return(nil, errors.New("internal error")).Once()
			},
			wantErr: true,
		},
		"success, no state filter": {
			request: &types.ListBatchOperationsRequest{Domain: testDomain},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.mockResource.VisibilityMgr.On("ListWorkflowExecutions", mock.Anything, mock.MatchedBy(func(req *persistence.ListWorkflowExecutionsByQueryRequest) bool {
					return req.Domain == constants.BatcherLocalDomainName && req.Query == "`Attr.CustomDomain` = 'test-domain'"
				})).Return(&persistence.ListWorkflowExecutionsResponse{
					Executions: []*types.WorkflowExecutionInfo{
						{
							Execution:        &types.WorkflowExecution{WorkflowID: "cadence-sys-batch-operation:test-domain:my-batch-id"},
							SearchAttributes: &types.SearchAttributes{IndexedFields: map[string][]byte{"Operator": operatorBytes}},
						},
					},
					NextPageToken: []byte("next"),
				}, nil).Once()
			},
			wantErr: false,
			validate: func(t *testing.T, resp *types.ListBatchOperationsResponse) {
				require.Len(t, resp.BatchOperations, 1)
				assert.Equal(t, "my-batch-id", resp.BatchOperations[0].BatchID)
				assert.Equal(t, types.BatchOperationStateRunning, resp.BatchOperations[0].State)
				assert.Equal(t, "my-operator", resp.BatchOperations[0].StartedBy)
				assert.Equal(t, []byte("next"), resp.NextPageToken)
			},
		},
		"success, skips legacy CLI batch without batch-operation prefix": {
			request: &types.ListBatchOperationsRequest{Domain: testDomain},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.mockResource.VisibilityMgr.On("ListWorkflowExecutions", mock.Anything, mock.Anything).
					Return(&persistence.ListWorkflowExecutionsResponse{
						Executions: []*types.WorkflowExecutionInfo{
							{Execution: &types.WorkflowExecution{WorkflowID: "some-random-legacy-cli-uuid"}},
							{Execution: &types.WorkflowExecution{WorkflowID: "cadence-sys-batch-operation:test-domain:my-batch-id"}},
						},
					}, nil).Once()
			},
			wantErr: false,
			validate: func(t *testing.T, resp *types.ListBatchOperationsResponse) {
				require.Len(t, resp.BatchOperations, 1)
				assert.Equal(t, "my-batch-id", resp.BatchOperations[0].BatchID)
			},
		},
		"success, with state filter": {
			request: &types.ListBatchOperationsRequest{Domain: testDomain, State: types.BatchOperationStateRunning},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.mockResource.VisibilityMgr.On("ListWorkflowExecutions", mock.Anything, mock.MatchedBy(func(req *persistence.ListWorkflowExecutionsByQueryRequest) bool {
					return req.Query == "`Attr.CustomDomain` = 'test-domain' and CloseTime = missing"
				})).Return(&persistence.ListWorkflowExecutionsResponse{}, nil).Once()
			},
			wantErr: false,
		},
		"success, pagination": {
			request: &types.ListBatchOperationsRequest{Domain: testDomain, PageSize: 10, NextPageToken: []byte("prev")},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.domainCache.EXPECT().GetDomainID(constants.BatcherLocalDomainName).Return(batcherDomainID, nil).AnyTimes()
				f.mockResource.VisibilityMgr.On("ListWorkflowExecutions", mock.Anything, mock.MatchedBy(func(req *persistence.ListWorkflowExecutionsByQueryRequest) bool {
					return req.PageSize == 10 && string(req.NextPageToken) == "prev"
				})).Return(&persistence.ListWorkflowExecutionsResponse{NextPageToken: []byte("next-page")}, nil).Once()
			},
			wantErr: false,
			validate: func(t *testing.T, resp *types.ListBatchOperationsResponse) {
				assert.Equal(t, []byte("next-page"), resp.NextPageToken)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newScheduleTestFixture(t)
			defer f.finish()
			tt.mockFn(f)

			resp, err := f.handler.ListBatchOperations(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, resp)
				if tt.validate != nil {
					tt.validate(t, resp)
				}
			}
		})
	}
}
