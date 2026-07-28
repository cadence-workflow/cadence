// Copyright (c) 2026 Uber Technologies Inc.
//
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
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/resource"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/workflowcache"
)

type asyncWorkflowTestHandler struct {
	handler             *handlerImpl
	mockShardController *shard.MockController
	mockAsyncMgr        *persistence.MockAsyncWorkflowQueueManager
}

func newAsyncWorkflowTestHandler(t *testing.T, ctrl *gomock.Controller) *asyncWorkflowTestHandler {
	mockResource := resource.NewTest(t, ctrl, metrics.History)
	mockResource.Logger = testlogger.New(t)
	mockShardController := shard.NewMockController(ctrl)
	mockWFCache := workflowcache.NewMockWFCache(ctrl)

	h := NewHandler(mockResource, config.NewForTest(), mockWFCache).(*handlerImpl)
	h.controller = mockShardController
	h.startWG.Done()

	// resource.NewTest wires GetAsyncWorkflowQueueManager to a single mock instance.
	mockAsyncMgr := mockResource.GetAsyncWorkflowQueueManager().(*persistence.MockAsyncWorkflowQueueManager)

	return &asyncWorkflowTestHandler{
		handler:             h,
		mockShardController: mockShardController,
		mockAsyncMgr:        mockAsyncMgr,
	}
}

// expectOwnershipLost sets up GetEngineForShard to fail the ownership guard.
func (h *asyncWorkflowTestHandler) expectOwnershipLost(shardID int) {
	h.mockShardController.EXPECT().GetEngineForShard(shardID).Return(nil, &types.ShardOwnershipLostError{}).Times(1)
}

// expectOwned sets up GetEngineForShard to pass the ownership guard.
func (h *asyncWorkflowTestHandler) expectOwned(shardID int) {
	h.mockShardController.EXPECT().GetEngineForShard(shardID).Return(nil, nil).Times(1)
}

func TestHandlerEnqueueAsyncWorkflowMessage(t *testing.T) {
	req := &types.EnqueueAsyncWorkflowMessageRequest{
		ShardID:      3,
		Payload:      []byte("payload"),
		Encoding:     "json",
		PartitionKey: "pk",
	}

	tests := []struct {
		name     string
		setup    func(h *asyncWorkflowTestHandler)
		wantResp *types.EnqueueAsyncWorkflowMessageResponse
		wantErr  bool
	}{
		{
			name: "ownership lost",
			setup: func(h *asyncWorkflowTestHandler) {
				h.expectOwnershipLost(3)
			},
			wantErr: true,
		},
		{
			name: "success",
			setup: func(h *asyncWorkflowTestHandler) {
				h.expectOwned(3)
				h.mockAsyncMgr.EXPECT().Enqueue(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, r *persistence.EnqueueAsyncWorkflowMessageRequest) (*persistence.EnqueueAsyncWorkflowMessageResponse, error) {
						assert.Equal(t, 3, r.ShardID)
						assert.Equal(t, []byte("payload"), r.Payload)
						assert.Equal(t, "json", r.Encoding)
						assert.Equal(t, "pk", r.PartitionKey)
						return &persistence.EnqueueAsyncWorkflowMessageResponse{MessageID: 42}, nil
					}).Times(1)
			},
			wantResp: &types.EnqueueAsyncWorkflowMessageResponse{MessageID: 42},
		},
		{
			name: "manager error",
			setup: func(h *asyncWorkflowTestHandler) {
				h.expectOwned(3)
				h.mockAsyncMgr.EXPECT().Enqueue(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			h := newAsyncWorkflowTestHandler(t, ctrl)
			tc.setup(h)

			resp, err := h.handler.EnqueueAsyncWorkflowMessage(context.Background(), req)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantResp, resp)
		})
	}
}
