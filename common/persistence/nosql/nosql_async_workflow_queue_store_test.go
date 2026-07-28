// The MIT License (MIT)
//
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

package nosql

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

const (
	testAsyncShardID = 7
)

type asyncQueueStoreTestData struct {
	mockDB *nosqlplugin.MockDB
}

func newAsyncQueueStoreTestData(t *testing.T) *asyncQueueStoreTestData {
	td := &asyncQueueStoreTestData{}
	ctrl := gomock.NewController(t)
	td.mockDB = nosqlplugin.NewMockDB(ctrl)

	mockPlugin := nosqlplugin.NewMockPlugin(ctrl)
	mockPlugin.EXPECT().CreateDB(gomock.Any(), gomock.Any(), gomock.Any()).Return(td.mockDB, nil).AnyTimes()
	RegisterPluginForTest(t, "cassandra", mockPlugin)
	return td
}

func (td *asyncQueueStoreTestData) newStore(t *testing.T) persistence.AsyncWorkflowQueueStore {
	store, err := newNoSQLAsyncWorkflowQueueStore(getValidShardedNoSQLConfig(), log.NewNoop(), metrics.NewNoopMetricsClient(), nil)
	require.NoError(t, err)
	require.NotNil(t, store)
	return store
}

func TestNewNoSQLAsyncWorkflowQueueStore_Succeeds(t *testing.T) {
	td := newAsyncQueueStoreTestData(t)
	td.newStore(t)
}

func TestAsyncWorkflowQueue_Enqueue(t *testing.T) {
	tests := []struct {
		name          string
		lastMessageID int64
		ackLevel      int64
		wantID        int64
	}{
		{name: "empty queue", lastMessageID: emptyMessageID, ackLevel: emptyMessageID, wantID: 0},
		{name: "last id ahead of ack", lastMessageID: 10, ackLevel: 3, wantID: 11},
		{name: "ack ahead of last id (after range delete)", lastMessageID: emptyMessageID, ackLevel: 8, wantID: 9},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			td := newAsyncQueueStoreTestData(t)
			store := td.newStore(t)
			ctx := context.Background()

			metadata := &nosqlplugin.AsyncWorkflowQueueMetadataRow{
				ShardID: testAsyncShardID, AckLevel: tc.ackLevel, Version: 2,
			}
			td.mockDB.EXPECT().SelectAsyncWorkflowQueueMetadata(ctx, testAsyncShardID).Return(metadata, nil).AnyTimes()
			td.mockDB.EXPECT().SelectLastAsyncWorkflowMessageID(ctx, testAsyncShardID).Return(tc.lastMessageID, nil)
			td.mockDB.EXPECT().InsertIntoAsyncWorkflowQueue(ctx, gomock.Any()).DoAndReturn(
				func(_ context.Context, row *nosqlplugin.AsyncWorkflowQueueMessageRow) error {
					assert.Equal(t, tc.wantID, row.ID)
					assert.Equal(t, testAsyncShardID, row.ShardID)
					return nil
				})

			resp, err := store.Enqueue(ctx, &persistence.EnqueueAsyncWorkflowMessageRequest{
				ShardID: testAsyncShardID, Payload: []byte("p"), Encoding: "thriftrw",
			})
			require.NoError(t, err)
			assert.Equal(t, tc.wantID, resp.MessageID)
		})
	}
}

func TestAsyncWorkflowQueue_EnqueueInitializesMetadataWhenMissing(t *testing.T) {
	td := newAsyncQueueStoreTestData(t)
	store := td.newStore(t)
	ctx := context.Background()
	notFound := errors.New("not found")

	// ensureQueueMetadata: first select misses, insert initial, then subsequent selects find it.
	gomock.InOrder(
		td.mockDB.EXPECT().SelectAsyncWorkflowQueueMetadata(ctx, testAsyncShardID).Return(nil, notFound),
		td.mockDB.EXPECT().IsNotFoundError(notFound).Return(true),
		td.mockDB.EXPECT().InsertAsyncWorkflowQueueMetadata(ctx, gomock.Any()).Return(nil),
		td.mockDB.EXPECT().SelectAsyncWorkflowQueueMetadata(ctx, testAsyncShardID).Return(
			&nosqlplugin.AsyncWorkflowQueueMetadataRow{AckLevel: emptyMessageID}, nil),
	)
	td.mockDB.EXPECT().SelectLastAsyncWorkflowMessageID(ctx, testAsyncShardID).Return(int64(emptyMessageID), nil)
	td.mockDB.EXPECT().InsertIntoAsyncWorkflowQueue(ctx, gomock.Any()).Return(nil)

	resp, err := store.Enqueue(ctx, &persistence.EnqueueAsyncWorkflowMessageRequest{ShardID: testAsyncShardID})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.MessageID)
}

func TestAsyncWorkflowQueue_EnqueueToDLQ(t *testing.T) {
	td := newAsyncQueueStoreTestData(t)
	store := td.newStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().SelectLastAsyncWorkflowDLQMessageID(ctx, testAsyncShardID).Return(int64(4), nil)
	td.mockDB.EXPECT().InsertIntoAsyncWorkflowDLQ(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, row *nosqlplugin.AsyncWorkflowQueueMessageRow) error {
			assert.Equal(t, int64(5), row.ID)
			return nil
		})

	resp, err := store.EnqueueToDLQ(ctx, &persistence.EnqueueAsyncWorkflowMessageRequest{ShardID: testAsyncShardID})
	require.NoError(t, err)
	assert.Equal(t, int64(5), resp.MessageID)
}

func TestAsyncWorkflowQueue_ReadMessages(t *testing.T) {
	td := newAsyncQueueStoreTestData(t)
	store := td.newStore(t)
	ctx := context.Background()

	rows := []*nosqlplugin.AsyncWorkflowQueueMessageRow{
		{ShardID: testAsyncShardID, ID: 1, Payload: []byte("a"), Encoding: "thriftrw"},
		{ShardID: testAsyncShardID, ID: 2, Payload: []byte("b"), Encoding: "thriftrw"},
	}
	td.mockDB.EXPECT().SelectAsyncWorkflowMessagesFrom(ctx, testAsyncShardID, int64(0), 10).Return(rows, nil)

	resp, err := store.ReadMessages(ctx, &persistence.ReadAsyncWorkflowMessagesRequest{
		ShardID: testAsyncShardID, LastMessageID: 0, PageSize: 10,
	})
	require.NoError(t, err)
	require.Len(t, resp.Messages, 2)
	assert.Equal(t, int64(1), resp.Messages[0].MessageID)
	assert.Equal(t, []byte("b"), resp.Messages[1].Payload)
}

func TestAsyncWorkflowQueue_UpdateAckLevel(t *testing.T) {
	tests := []struct {
		name        string
		current     int64
		requested   int64
		expectCAS   bool
		wantVersion int64
	}{
		{name: "advances", current: 5, requested: 9, expectCAS: true, wantVersion: 4},
		{name: "no backwards move", current: 9, requested: 5, expectCAS: false},
		{name: "no-op when equal", current: 9, requested: 9, expectCAS: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			td := newAsyncQueueStoreTestData(t)
			store := td.newStore(t)
			ctx := context.Background()

			metadata := &nosqlplugin.AsyncWorkflowQueueMetadataRow{
				ShardID: testAsyncShardID, AckLevel: tc.current, Version: tc.wantVersion,
			}
			td.mockDB.EXPECT().SelectAsyncWorkflowQueueMetadata(ctx, testAsyncShardID).Return(metadata, nil).AnyTimes()
			if tc.expectCAS {
				td.mockDB.EXPECT().UpdateAsyncWorkflowQueueMetadataCas(ctx, gomock.Any()).DoAndReturn(
					func(_ context.Context, row nosqlplugin.AsyncWorkflowQueueMetadataRow) error {
						assert.Equal(t, tc.requested, row.AckLevel)
						assert.Equal(t, tc.wantVersion+1, row.Version)
						return nil
					})
			}

			err := store.UpdateAckLevel(ctx, &persistence.UpdateAsyncWorkflowAckLevelRequest{
				ShardID: testAsyncShardID, AckLevel: tc.requested,
			})
			require.NoError(t, err)
		})
	}
}

func TestAsyncWorkflowQueue_GetAckLevel_NotFound(t *testing.T) {
	td := newAsyncQueueStoreTestData(t)
	store := td.newStore(t)
	ctx := context.Background()
	notFound := errors.New("not found")

	td.mockDB.EXPECT().SelectAsyncWorkflowQueueMetadata(ctx, testAsyncShardID).Return(nil, notFound)
	td.mockDB.EXPECT().IsNotFoundError(notFound).Return(true)

	resp, err := store.GetAckLevel(ctx, &persistence.GetAsyncWorkflowAckLevelRequest{ShardID: testAsyncShardID})
	require.NoError(t, err)
	assert.Equal(t, int64(emptyMessageID), resp.AckLevel)
}

func TestAsyncWorkflowQueue_RangeDelete(t *testing.T) {
	td := newAsyncQueueStoreTestData(t)
	store := td.newStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().RangeDeleteAsyncWorkflowMessages(ctx, testAsyncShardID, int64(20)).Return(nil)
	require.NoError(t, store.RangeDeleteMessages(ctx, &persistence.RangeDeleteAsyncWorkflowMessagesRequest{
		ShardID: testAsyncShardID, InclusiveEndMessageID: 20,
	}))

	td.mockDB.EXPECT().RangeDeleteAsyncWorkflowDLQMessages(ctx, testAsyncShardID, int64(30)).Return(nil)
	require.NoError(t, store.RangeDeleteMessagesFromDLQ(ctx, &persistence.RangeDeleteAsyncWorkflowMessagesRequest{
		ShardID: testAsyncShardID, InclusiveEndMessageID: 30,
	}))
}

func TestGetNextAsyncWorkflowID(t *testing.T) {
	tests := []struct {
		lastMessageID int64
		ackLevel      int64
		want          int64
	}{
		{lastMessageID: emptyMessageID, ackLevel: emptyMessageID, want: 0},
		{lastMessageID: 5, ackLevel: 2, want: 6},
		{lastMessageID: 2, ackLevel: 5, want: 6},
		{lastMessageID: 5, ackLevel: 5, want: 6},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, getNextAsyncWorkflowID(tc.lastMessageID, tc.ackLevel))
	}
}
