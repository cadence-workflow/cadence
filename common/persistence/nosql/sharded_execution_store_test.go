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

package nosql

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/types"
)

func TestShardedExecutionStore_NilShardIDReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := NewMockshardedNosqlStore(ctrl)
	mockSerializer := serialization.NewMockTaskSerializer(ctrl)

	store := NewShardedExecutionStore(mockStore, log.NewNoop(), mockSerializer)

	_, err := store.GetWorkflowExecution(context.Background(), &persistence.InternalGetWorkflowExecutionRequest{
		ShardID: nil,
	})
	require.Error(t, err)
	var badReq *types.BadRequestError
	assert.True(t, errors.As(err, &badReq), "expected BadRequestError, got %T: %v", err, err)
}

func TestShardedExecutionStore_ShardStoreIsCached(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := NewMockshardedNosqlStore(ctrl)
	mockDB := nosqlplugin.NewMockDB(ctrl)
	mockSerializer := serialization.NewMockTaskSerializer(ctrl)

	shardID := 5
	nosqlShard := &nosqlStore{db: mockDB, logger: log.NewNoop(), dc: &persistence.DynamicConfiguration{}}

	// GetStoreShardByHistoryShard must be called exactly once despite two requests for the same shard.
	mockStore.EXPECT().GetStoreShardByHistoryShard(shardID).Return(nosqlShard, nil).Times(1)
	mockDB.EXPECT().PluginName().Return("test").AnyTimes()

	// Two distinct calls that both require shardID=5.
	mockDB.EXPECT().SelectWorkflowExecution(gomock.Any(), shardID, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("not found")).Times(2)
	mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(2)

	s := NewShardedExecutionStore(mockStore, log.NewNoop(), mockSerializer)

	req := &persistence.InternalGetWorkflowExecutionRequest{
		ShardID:   &shardID,
		DomainID:  "dom",
		Execution: types.WorkflowExecution{WorkflowID: "wf", RunID: "run"},
	}
	_, _ = s.GetWorkflowExecution(context.Background(), req)
	_, _ = s.GetWorkflowExecution(context.Background(), req)
	// If the store were re-created on each call, GetStoreShardByHistoryShard would be called twice
	// and the mock would fail with an unexpected call.
}

func TestShardedExecutionStore_GetStoreShardError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := NewMockshardedNosqlStore(ctrl)
	mockSerializer := serialization.NewMockTaskSerializer(ctrl)

	shardID := 3
	mockStore.EXPECT().GetStoreShardByHistoryShard(shardID).Return(nil, errors.New("no shard config"))

	s := NewShardedExecutionStore(mockStore, log.NewNoop(), mockSerializer)
	_, err := s.GetWorkflowExecution(context.Background(), &persistence.InternalGetWorkflowExecutionRequest{
		ShardID: &shardID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no shard config")
}
