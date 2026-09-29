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

package persistence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

// asyncWorkflowTestNow uses a non-UTC zone on purpose: the manager must stamp UTC.
var asyncWorkflowTestNow = time.Date(2026, 9, 28, 12, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))

var errAsyncWorkflowStore = errors.New("async workflow store failure")

// asyncWorkflowCase is one table row. mutate edits a known-valid request; rows that
// set wantBadRequest must produce *types.BadRequestError and must NOT reach the store.
type asyncWorkflowCase[R any] struct {
	name           string
	mutate         func(*R)
	storeErr       error
	wantBadRequest bool
}

func newTestAsyncWorkflowQueueManager(t *testing.T, store AsyncWorkflowQueueStore) *asyncWorkflowQueueManagerImpl {
	return &asyncWorkflowQueueManagerImpl{
		persistence: store,
		logger:      testlogger.New(t),
		timeSrc:     clock.NewMockedTimeSourceAt(asyncWorkflowTestNow),
	}
}

func assertAsyncWorkflowErr(t *testing.T, err error, wantBadRequest bool, storeErr error) {
	t.Helper()
	switch {
	case wantBadRequest:
		var badRequest *types.BadRequestError
		require.True(t, errors.As(err, &badRequest), "expected *types.BadRequestError, got %T: %v", err, err)
		assert.NotEmpty(t, badRequest.Message)
	case storeErr != nil:
		assert.True(t, err == storeErr, "store error must pass through unchanged, got %v", err)
	default:
		require.NoError(t, err)
	}
}

func assertAsyncWorkflowStamped(t *testing.T, got time.Time) {
	t.Helper()
	assert.True(t, asyncWorkflowTestNow.Equal(got), "CurrentTimeStamp = %v, want the mocked clock time %v", got, asyncWorkflowTestNow)
	assert.Equal(t, time.UTC, got.Location(), "CurrentTimeStamp must be in UTC")
}

func newValidAsyncWorkflowMessage() *AsyncWorkflowMessage {
	return &AsyncWorkflowMessage{
		ShardID:         3,
		SourceCluster:   "cluster-a",
		MessageID:       101,
		DomainName:      "test-domain",
		WorkflowID:      "test-workflow",
		RequestID:       "test-request",
		RequestType:     AsyncWorkflowRequestTypeSignalWithStartWorkflow,
		Payload:         []byte("payload"),
		PayloadEncoding: constants.EncodingTypeJSON,
		CreatedTime:     asyncWorkflowTestNow.Add(-time.Minute),
	}
}

func newValidEnqueueAsyncWorkflowMessageRequest() *EnqueueAsyncWorkflowMessageRequest {
	return &EnqueueAsyncWorkflowMessageRequest{
		ShardID:         3,
		SourceCluster:   "cluster-a",
		MessageID:       101,
		DomainName:      "test-domain",
		WorkflowID:      "test-workflow",
		RequestID:       "test-request",
		RequestType:     AsyncWorkflowRequestTypeStartWorkflow,
		Payload:         []byte("payload"),
		PayloadEncoding: constants.EncodingTypeJSON,
	}
}

func TestNewAsyncWorkflowQueueManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAsyncWorkflowQueueStore(ctrl)
	store.EXPECT().GetName().Return("test-store").Times(1)

	mgr := NewAsyncWorkflowQueueManager(store, testlogger.New(t))

	require.NotNil(t, mgr)
	assert.Equal(t, "test-store", mgr.GetName())
}

func TestAsyncWorkflowQueueManager_GetName(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAsyncWorkflowQueueStore(ctrl)
	store.EXPECT().GetName().Return("cassandra").Times(1)

	mgr := newTestAsyncWorkflowQueueManager(t, store)

	assert.Equal(t, "cassandra", mgr.GetName())
}

func TestAsyncWorkflowQueueManager_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAsyncWorkflowQueueStore(ctrl)
	store.EXPECT().Close().Times(1)

	mgr := newTestAsyncWorkflowQueueManager(t, store)

	mgr.Close()
}

func TestAsyncWorkflowQueueManager_EnqueueAsyncWorkflowMessage(t *testing.T) {
	tests := []asyncWorkflowCase[EnqueueAsyncWorkflowMessageRequest]{
		{name: "valid start workflow request"},
		{
			name: "valid signal with start request",
			mutate: func(r *EnqueueAsyncWorkflowMessageRequest) {
				r.RequestType = AsyncWorkflowRequestTypeSignalWithStartWorkflow
			},
		},
		{
			name:   "valid shard id zero",
			mutate: func(r *EnqueueAsyncWorkflowMessageRequest) { r.ShardID = 0 },
		},
		{
			name:     "store error passes through unchanged",
			storeErr: errAsyncWorkflowStore,
		},
		{
			name:           "negative shard id",
			mutate:         func(r *EnqueueAsyncWorkflowMessageRequest) { r.ShardID = -1 },
			wantBadRequest: true,
		},
		{
			name:           "empty source cluster",
			mutate:         func(r *EnqueueAsyncWorkflowMessageRequest) { r.SourceCluster = "" },
			wantBadRequest: true,
		},
		{
			name:           "zero message id",
			mutate:         func(r *EnqueueAsyncWorkflowMessageRequest) { r.MessageID = 0 },
			wantBadRequest: true,
		},
		{
			name:           "negative message id",
			mutate:         func(r *EnqueueAsyncWorkflowMessageRequest) { r.MessageID = -5 },
			wantBadRequest: true,
		},
		{
			name:           "empty domain name",
			mutate:         func(r *EnqueueAsyncWorkflowMessageRequest) { r.DomainName = "" },
			wantBadRequest: true,
		},
		{
			name:           "empty workflow id",
			mutate:         func(r *EnqueueAsyncWorkflowMessageRequest) { r.WorkflowID = "" },
			wantBadRequest: true,
		},
		{
			name:           "empty request id",
			mutate:         func(r *EnqueueAsyncWorkflowMessageRequest) { r.RequestID = "" },
			wantBadRequest: true,
		},
		{
			name:           "nil payload",
			mutate:         func(r *EnqueueAsyncWorkflowMessageRequest) { r.Payload = nil },
			wantBadRequest: true,
		},
		{
			name:           "empty payload",
			mutate:         func(r *EnqueueAsyncWorkflowMessageRequest) { r.Payload = []byte{} },
			wantBadRequest: true,
		},
		{
			name:           "empty payload encoding",
			mutate:         func(r *EnqueueAsyncWorkflowMessageRequest) { r.PayloadEncoding = "" },
			wantBadRequest: true,
		},
		{
			name:           "request type above range",
			mutate:         func(r *EnqueueAsyncWorkflowMessageRequest) { r.RequestType = AsyncWorkflowRequestType(2) },
			wantBadRequest: true,
		},
		{
			name:           "request type below range",
			mutate:         func(r *EnqueueAsyncWorkflowMessageRequest) { r.RequestType = AsyncWorkflowRequestType(-1) },
			wantBadRequest: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockAsyncWorkflowQueueStore(ctrl)
			mgr := newTestAsyncWorkflowQueueManager(t, store)

			req := newValidEnqueueAsyncWorkflowMessageRequest()
			if tc.mutate != nil {
				tc.mutate(req)
			}
			if !tc.wantBadRequest {
				store.EXPECT().EnqueueAsyncWorkflowMessage(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, got *InternalEnqueueAsyncWorkflowMessageRequest) error {
						assert.Same(t, req, got.EnqueueAsyncWorkflowMessageRequest)
						assertAsyncWorkflowStamped(t, got.CurrentTimeStamp)
						return tc.storeErr
					}).Times(1)
			}

			err := mgr.EnqueueAsyncWorkflowMessage(context.Background(), req)

			assertAsyncWorkflowErr(t, err, tc.wantBadRequest, tc.storeErr)
		})
	}
}

func TestAsyncWorkflowQueueManager_ReadAsyncWorkflowMessages(t *testing.T) {
	tests := []asyncWorkflowCase[ReadAsyncWorkflowMessagesRequest]{
		{name: "valid request"},
		{
			name:   "valid exclusive min message id zero",
			mutate: func(r *ReadAsyncWorkflowMessagesRequest) { r.ExclusiveMinMessageID = 0 },
		},
		{
			name:   "valid shard id zero",
			mutate: func(r *ReadAsyncWorkflowMessagesRequest) { r.ShardID = 0 },
		},
		{
			name:     "store error passes through unchanged",
			storeErr: errAsyncWorkflowStore,
		},
		{
			name:           "negative shard id",
			mutate:         func(r *ReadAsyncWorkflowMessagesRequest) { r.ShardID = -1 },
			wantBadRequest: true,
		},
		{
			name:           "empty source cluster",
			mutate:         func(r *ReadAsyncWorkflowMessagesRequest) { r.SourceCluster = "" },
			wantBadRequest: true,
		},
		{
			name:           "zero page size",
			mutate:         func(r *ReadAsyncWorkflowMessagesRequest) { r.PageSize = 0 },
			wantBadRequest: true,
		},
		{
			name:           "negative page size",
			mutate:         func(r *ReadAsyncWorkflowMessagesRequest) { r.PageSize = -10 },
			wantBadRequest: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockAsyncWorkflowQueueStore(ctrl)
			mgr := newTestAsyncWorkflowQueueManager(t, store)

			req := &ReadAsyncWorkflowMessagesRequest{
				ShardID:               3,
				SourceCluster:         "cluster-a",
				ExclusiveMinMessageID: 100,
				PageSize:              50,
			}
			if tc.mutate != nil {
				tc.mutate(req)
			}
			wantResp := &ReadAsyncWorkflowMessagesResponse{Messages: []*AsyncWorkflowMessage{newValidAsyncWorkflowMessage()}}
			if !tc.wantBadRequest {
				store.EXPECT().ReadAsyncWorkflowMessages(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, got *ReadAsyncWorkflowMessagesRequest) (*ReadAsyncWorkflowMessagesResponse, error) {
						assert.Same(t, req, got)
						if tc.storeErr != nil {
							return nil, tc.storeErr
						}
						return wantResp, nil
					}).Times(1)
			}

			gotResp, err := mgr.ReadAsyncWorkflowMessages(context.Background(), req)

			assertAsyncWorkflowErr(t, err, tc.wantBadRequest, tc.storeErr)
			if tc.wantBadRequest || tc.storeErr != nil {
				assert.Nil(t, gotResp)
			} else {
				assert.Same(t, wantResp, gotResp)
			}
		})
	}
}

func TestAsyncWorkflowQueueManager_GetAsyncWorkflowAckLevels(t *testing.T) {
	tests := []asyncWorkflowCase[GetAsyncWorkflowAckLevelsRequest]{
		{name: "valid request"},
		{
			name:   "valid shard id zero",
			mutate: func(r *GetAsyncWorkflowAckLevelsRequest) { r.ShardID = 0 },
		},
		{
			name:     "store error passes through unchanged",
			storeErr: errAsyncWorkflowStore,
		},
		{
			name:           "negative shard id",
			mutate:         func(r *GetAsyncWorkflowAckLevelsRequest) { r.ShardID = -1 },
			wantBadRequest: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockAsyncWorkflowQueueStore(ctrl)
			mgr := newTestAsyncWorkflowQueueManager(t, store)

			req := &GetAsyncWorkflowAckLevelsRequest{ShardID: 3}
			if tc.mutate != nil {
				tc.mutate(req)
			}
			wantResp := &GetAsyncWorkflowAckLevelsResponse{AckLevels: map[string]int64{"cluster-a": 90, "cluster-b": 12}}
			if !tc.wantBadRequest {
				store.EXPECT().GetAsyncWorkflowAckLevels(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, got *GetAsyncWorkflowAckLevelsRequest) (*GetAsyncWorkflowAckLevelsResponse, error) {
						assert.Same(t, req, got)
						if tc.storeErr != nil {
							return nil, tc.storeErr
						}
						return wantResp, nil
					}).Times(1)
			}

			gotResp, err := mgr.GetAsyncWorkflowAckLevels(context.Background(), req)

			assertAsyncWorkflowErr(t, err, tc.wantBadRequest, tc.storeErr)
			if tc.wantBadRequest || tc.storeErr != nil {
				assert.Nil(t, gotResp)
			} else {
				assert.Same(t, wantResp, gotResp)
			}
		})
	}
}

func TestAsyncWorkflowQueueManager_UpdateAsyncWorkflowAckLevel(t *testing.T) {
	tests := []asyncWorkflowCase[UpdateAsyncWorkflowAckLevelRequest]{
		{name: "valid request"},
		{
			name:   "valid ack level zero",
			mutate: func(r *UpdateAsyncWorkflowAckLevelRequest) { r.AckLevel = 0 },
		},
		{
			name:   "valid shard id zero",
			mutate: func(r *UpdateAsyncWorkflowAckLevelRequest) { r.ShardID = 0 },
		},
		{
			name:     "store error passes through unchanged",
			storeErr: errAsyncWorkflowStore,
		},
		{
			name:           "negative shard id",
			mutate:         func(r *UpdateAsyncWorkflowAckLevelRequest) { r.ShardID = -1 },
			wantBadRequest: true,
		},
		{
			name:           "empty source cluster",
			mutate:         func(r *UpdateAsyncWorkflowAckLevelRequest) { r.SourceCluster = "" },
			wantBadRequest: true,
		},
		{
			name:           "negative ack level",
			mutate:         func(r *UpdateAsyncWorkflowAckLevelRequest) { r.AckLevel = -1 },
			wantBadRequest: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockAsyncWorkflowQueueStore(ctrl)
			mgr := newTestAsyncWorkflowQueueManager(t, store)

			req := &UpdateAsyncWorkflowAckLevelRequest{ShardID: 3, SourceCluster: "cluster-a", AckLevel: 500}
			if tc.mutate != nil {
				tc.mutate(req)
			}
			if !tc.wantBadRequest {
				store.EXPECT().UpdateAsyncWorkflowAckLevel(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, got *InternalUpdateAsyncWorkflowAckLevelRequest) error {
						assert.Same(t, req, got.UpdateAsyncWorkflowAckLevelRequest)
						assertAsyncWorkflowStamped(t, got.CurrentTimeStamp)
						return tc.storeErr
					}).Times(1)
			}

			err := mgr.UpdateAsyncWorkflowAckLevel(context.Background(), req)

			assertAsyncWorkflowErr(t, err, tc.wantBadRequest, tc.storeErr)
		})
	}
}

func TestAsyncWorkflowQueueManager_RangeDeleteAsyncWorkflowMessages(t *testing.T) {
	tests := []asyncWorkflowCase[RangeDeleteAsyncWorkflowMessagesRequest]{
		{name: "valid request"},
		{
			name:   "valid inclusive max message id zero",
			mutate: func(r *RangeDeleteAsyncWorkflowMessagesRequest) { r.InclusiveMaxMessageID = 0 },
		},
		{
			name:   "valid shard id zero",
			mutate: func(r *RangeDeleteAsyncWorkflowMessagesRequest) { r.ShardID = 0 },
		},
		{
			name:     "store error passes through unchanged",
			storeErr: errAsyncWorkflowStore,
		},
		{
			name:           "negative shard id",
			mutate:         func(r *RangeDeleteAsyncWorkflowMessagesRequest) { r.ShardID = -1 },
			wantBadRequest: true,
		},
		{
			name:           "empty source cluster",
			mutate:         func(r *RangeDeleteAsyncWorkflowMessagesRequest) { r.SourceCluster = "" },
			wantBadRequest: true,
		},
		{
			name:           "negative inclusive max message id",
			mutate:         func(r *RangeDeleteAsyncWorkflowMessagesRequest) { r.InclusiveMaxMessageID = -1 },
			wantBadRequest: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockAsyncWorkflowQueueStore(ctrl)
			mgr := newTestAsyncWorkflowQueueManager(t, store)

			req := &RangeDeleteAsyncWorkflowMessagesRequest{ShardID: 3, SourceCluster: "cluster-a", InclusiveMaxMessageID: 500}
			if tc.mutate != nil {
				tc.mutate(req)
			}
			if !tc.wantBadRequest {
				store.EXPECT().RangeDeleteAsyncWorkflowMessages(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, got *RangeDeleteAsyncWorkflowMessagesRequest) error {
						assert.Same(t, req, got)
						return tc.storeErr
					}).Times(1)
			}

			err := mgr.RangeDeleteAsyncWorkflowMessages(context.Background(), req)

			assertAsyncWorkflowErr(t, err, tc.wantBadRequest, tc.storeErr)
		})
	}
}

func TestAsyncWorkflowQueueManager_EnqueueAsyncWorkflowMessageToDLQ(t *testing.T) {
	tests := []asyncWorkflowCase[EnqueueAsyncWorkflowMessageToDLQRequest]{
		{name: "valid request"},
		{
			name: "valid start workflow message",
			mutate: func(r *EnqueueAsyncWorkflowMessageToDLQRequest) {
				r.Message.RequestType = AsyncWorkflowRequestTypeStartWorkflow
			},
		},
		{
			name:     "store error passes through unchanged",
			storeErr: errAsyncWorkflowStore,
		},
		{
			name:           "nil message",
			mutate:         func(r *EnqueueAsyncWorkflowMessageToDLQRequest) { r.Message = nil },
			wantBadRequest: true,
		},
		{
			name:           "empty reason",
			mutate:         func(r *EnqueueAsyncWorkflowMessageToDLQRequest) { r.Reason = "" },
			wantBadRequest: true,
		},
		{
			name:           "message negative shard id",
			mutate:         func(r *EnqueueAsyncWorkflowMessageToDLQRequest) { r.Message.ShardID = -1 },
			wantBadRequest: true,
		},
		{
			name:           "message empty source cluster",
			mutate:         func(r *EnqueueAsyncWorkflowMessageToDLQRequest) { r.Message.SourceCluster = "" },
			wantBadRequest: true,
		},
		{
			name:           "message zero message id",
			mutate:         func(r *EnqueueAsyncWorkflowMessageToDLQRequest) { r.Message.MessageID = 0 },
			wantBadRequest: true,
		},
		{
			name:           "message empty domain name",
			mutate:         func(r *EnqueueAsyncWorkflowMessageToDLQRequest) { r.Message.DomainName = "" },
			wantBadRequest: true,
		},
		{
			name:           "message empty workflow id",
			mutate:         func(r *EnqueueAsyncWorkflowMessageToDLQRequest) { r.Message.WorkflowID = "" },
			wantBadRequest: true,
		},
		{
			name:           "message empty request id",
			mutate:         func(r *EnqueueAsyncWorkflowMessageToDLQRequest) { r.Message.RequestID = "" },
			wantBadRequest: true,
		},
		{
			name:           "message empty payload",
			mutate:         func(r *EnqueueAsyncWorkflowMessageToDLQRequest) { r.Message.Payload = nil },
			wantBadRequest: true,
		},
		{
			name:           "message empty payload encoding",
			mutate:         func(r *EnqueueAsyncWorkflowMessageToDLQRequest) { r.Message.PayloadEncoding = "" },
			wantBadRequest: true,
		},
		{
			name:           "message request type out of range",
			mutate:         func(r *EnqueueAsyncWorkflowMessageToDLQRequest) { r.Message.RequestType = AsyncWorkflowRequestType(2) },
			wantBadRequest: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockAsyncWorkflowQueueStore(ctrl)
			mgr := newTestAsyncWorkflowQueueManager(t, store)

			req := &EnqueueAsyncWorkflowMessageToDLQRequest{
				Message: newValidAsyncWorkflowMessage(),
				Reason:  "workflow start failed after max retries",
			}
			if tc.mutate != nil {
				tc.mutate(req)
			}
			if !tc.wantBadRequest {
				store.EXPECT().EnqueueAsyncWorkflowMessageToDLQ(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, got *InternalEnqueueAsyncWorkflowMessageToDLQRequest) error {
						assert.Same(t, req, got.EnqueueAsyncWorkflowMessageToDLQRequest)
						assertAsyncWorkflowStamped(t, got.CurrentTimeStamp)
						return tc.storeErr
					}).Times(1)
			}

			err := mgr.EnqueueAsyncWorkflowMessageToDLQ(context.Background(), req)

			assertAsyncWorkflowErr(t, err, tc.wantBadRequest, tc.storeErr)
		})
	}
}

func TestAsyncWorkflowQueueManager_ReadAsyncWorkflowMessagesFromDLQ(t *testing.T) {
	tests := []asyncWorkflowCase[ReadAsyncWorkflowMessagesFromDLQRequest]{
		{name: "valid request"},
		{
			name:   "valid exclusive min message id zero",
			mutate: func(r *ReadAsyncWorkflowMessagesFromDLQRequest) { r.ExclusiveMinMessageID = 0 },
		},
		{
			name:   "valid shard id zero",
			mutate: func(r *ReadAsyncWorkflowMessagesFromDLQRequest) { r.ShardID = 0 },
		},
		{
			name:     "store error passes through unchanged",
			storeErr: errAsyncWorkflowStore,
		},
		{
			name:           "negative shard id",
			mutate:         func(r *ReadAsyncWorkflowMessagesFromDLQRequest) { r.ShardID = -1 },
			wantBadRequest: true,
		},
		{
			name:           "empty source cluster",
			mutate:         func(r *ReadAsyncWorkflowMessagesFromDLQRequest) { r.SourceCluster = "" },
			wantBadRequest: true,
		},
		{
			name:           "zero page size",
			mutate:         func(r *ReadAsyncWorkflowMessagesFromDLQRequest) { r.PageSize = 0 },
			wantBadRequest: true,
		},
		{
			name:           "negative page size",
			mutate:         func(r *ReadAsyncWorkflowMessagesFromDLQRequest) { r.PageSize = -10 },
			wantBadRequest: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockAsyncWorkflowQueueStore(ctrl)
			mgr := newTestAsyncWorkflowQueueManager(t, store)

			req := &ReadAsyncWorkflowMessagesFromDLQRequest{
				ShardID:               3,
				SourceCluster:         "cluster-a",
				ExclusiveMinMessageID: 100,
				PageSize:              50,
			}
			if tc.mutate != nil {
				tc.mutate(req)
			}
			wantResp := &ReadAsyncWorkflowMessagesFromDLQResponse{
				Messages: []*AsyncWorkflowDLQMessage{{AsyncWorkflowMessage: *newValidAsyncWorkflowMessage(), Reason: "poison message"}},
			}
			if !tc.wantBadRequest {
				store.EXPECT().ReadAsyncWorkflowMessagesFromDLQ(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, got *ReadAsyncWorkflowMessagesFromDLQRequest) (*ReadAsyncWorkflowMessagesFromDLQResponse, error) {
						assert.Same(t, req, got)
						if tc.storeErr != nil {
							return nil, tc.storeErr
						}
						return wantResp, nil
					}).Times(1)
			}

			gotResp, err := mgr.ReadAsyncWorkflowMessagesFromDLQ(context.Background(), req)

			assertAsyncWorkflowErr(t, err, tc.wantBadRequest, tc.storeErr)
			if tc.wantBadRequest || tc.storeErr != nil {
				assert.Nil(t, gotResp)
			} else {
				assert.Same(t, wantResp, gotResp)
			}
		})
	}
}

func TestAsyncWorkflowQueueManager_RangeDeleteAsyncWorkflowMessagesFromDLQ(t *testing.T) {
	tests := []asyncWorkflowCase[RangeDeleteAsyncWorkflowMessagesFromDLQRequest]{
		{name: "valid request"},
		{
			name:   "valid inclusive max message id zero",
			mutate: func(r *RangeDeleteAsyncWorkflowMessagesFromDLQRequest) { r.InclusiveMaxMessageID = 0 },
		},
		{
			name:   "valid shard id zero",
			mutate: func(r *RangeDeleteAsyncWorkflowMessagesFromDLQRequest) { r.ShardID = 0 },
		},
		{
			name:     "store error passes through unchanged",
			storeErr: errAsyncWorkflowStore,
		},
		{
			name:           "negative shard id",
			mutate:         func(r *RangeDeleteAsyncWorkflowMessagesFromDLQRequest) { r.ShardID = -1 },
			wantBadRequest: true,
		},
		{
			name:           "empty source cluster",
			mutate:         func(r *RangeDeleteAsyncWorkflowMessagesFromDLQRequest) { r.SourceCluster = "" },
			wantBadRequest: true,
		},
		{
			name:           "negative inclusive max message id",
			mutate:         func(r *RangeDeleteAsyncWorkflowMessagesFromDLQRequest) { r.InclusiveMaxMessageID = -1 },
			wantBadRequest: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockAsyncWorkflowQueueStore(ctrl)
			mgr := newTestAsyncWorkflowQueueManager(t, store)

			req := &RangeDeleteAsyncWorkflowMessagesFromDLQRequest{ShardID: 3, SourceCluster: "cluster-a", InclusiveMaxMessageID: 500}
			if tc.mutate != nil {
				tc.mutate(req)
			}
			if !tc.wantBadRequest {
				store.EXPECT().RangeDeleteAsyncWorkflowMessagesFromDLQ(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, got *RangeDeleteAsyncWorkflowMessagesFromDLQRequest) error {
						assert.Same(t, req, got)
						return tc.storeErr
					}).Times(1)
			}

			err := mgr.RangeDeleteAsyncWorkflowMessagesFromDLQ(context.Background(), req)

			assertAsyncWorkflowErr(t, err, tc.wantBadRequest, tc.storeErr)
		})
	}
}
