// Copyright (c) 2026 Uber Technologies, Inc.
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

						var params batcher.BatchParams
						require.NoError(t, json.Unmarshal(req.StartRequest.Input, &params))
						assert.Equal(t, testDomain, params.DomainName)
						assert.Equal(t, "WorkflowType = 'my-workflow-type'", params.Query)
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
