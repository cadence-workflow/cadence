package nosql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

func newTestAsyncWorkflowQueueStore(t *testing.T) (*nosqlAsyncWorkflowQueueStore, *nosqlplugin.MockDB, *MockshardedNosqlStore, *nosqlStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	db := nosqlplugin.NewMockDB(ctrl)
	sharded := NewMockshardedNosqlStore(ctrl)
	storeShard := &nosqlStore{db: db, logger: testlogger.New(t)}
	return &nosqlAsyncWorkflowQueueStore{shardedNosqlStore: sharded}, db, sharded, storeShard
}

func TestNoSQLAsyncWorkflowQueueStoreMappings(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2025, 1, 6, 15, 0, 0, 0, time.UTC)
	row := &nosqlplugin.AsyncWorkflowMessageRow{
		ShardID:         3,
		SourceCluster:   "c1",
		MessageID:       11,
		DomainName:      "domain",
		WorkflowID:      "workflow",
		RequestID:       "request",
		RequestType:     1,
		Payload:         []byte("payload"),
		PayloadEncoding: "thriftrw",
		CreatedTime:     now,
	}

	t.Run("enqueue", func(t *testing.T) {
		store, db, sharded, storeShard := newTestAsyncWorkflowQueueStore(t)
		sharded.EXPECT().GetStoreShardByHistoryShard(3).Return(storeShard, nil)
		db.EXPECT().InsertAsyncWorkflowMessage(ctx, row).Return(nil)

		err := store.EnqueueAsyncWorkflowMessage(ctx, &persistence.InternalEnqueueAsyncWorkflowMessageRequest{
			EnqueueAsyncWorkflowMessageRequest: &persistence.EnqueueAsyncWorkflowMessageRequest{
				ShardID:         3,
				SourceCluster:   "c1",
				MessageID:       11,
				DomainName:      "domain",
				WorkflowID:      "workflow",
				RequestID:       "request",
				RequestType:     persistence.AsyncWorkflowRequestTypeSignalWithStartWorkflow,
				Payload:         []byte("payload"),
				PayloadEncoding: constants.EncodingTypeThriftRW,
			},
			CurrentTimeStamp: now,
		})
		require.NoError(t, err)
	})

	t.Run("read", func(t *testing.T) {
		store, db, sharded, storeShard := newTestAsyncWorkflowQueueStore(t)
		sharded.EXPECT().GetStoreShardByHistoryShard(3).Return(storeShard, nil)
		db.EXPECT().SelectAsyncWorkflowMessages(ctx, 3, "c1", int64(10), 5).Return([]*nosqlplugin.AsyncWorkflowMessageRow{row}, nil)

		response, err := store.ReadAsyncWorkflowMessages(ctx, &persistence.ReadAsyncWorkflowMessagesRequest{
			ShardID: 3, SourceCluster: "c1", ExclusiveMinMessageID: 10, PageSize: 5,
		})
		require.NoError(t, err)
		require.Len(t, response.Messages, 1)
		assert.Equal(t, &persistence.AsyncWorkflowMessage{
			ShardID: 3, SourceCluster: "c1", MessageID: 11, DomainName: "domain",
			WorkflowID: "workflow", RequestID: "request", RequestType: persistence.AsyncWorkflowRequestTypeSignalWithStartWorkflow,
			Payload: []byte("payload"), PayloadEncoding: constants.EncodingTypeThriftRW, CreatedTime: now,
		}, response.Messages[0])
	})

	t.Run("ack levels are non nil", func(t *testing.T) {
		store, db, sharded, storeShard := newTestAsyncWorkflowQueueStore(t)
		sharded.EXPECT().GetStoreShardByHistoryShard(3).Return(storeShard, nil)
		db.EXPECT().SelectAsyncWorkflowAckLevels(ctx, 3).Return(nil, nil)

		response, err := store.GetAsyncWorkflowAckLevels(ctx, &persistence.GetAsyncWorkflowAckLevelsRequest{ShardID: 3})
		require.NoError(t, err)
		require.NotNil(t, response.AckLevels)
		assert.Empty(t, response.AckLevels)
	})

	t.Run("update ack", func(t *testing.T) {
		store, db, sharded, storeShard := newTestAsyncWorkflowQueueStore(t)
		sharded.EXPECT().GetStoreShardByHistoryShard(3).Return(storeShard, nil)
		db.EXPECT().UpsertAsyncWorkflowAckLevel(ctx, 3, "c1", int64(40), now).Return(nil)

		err := store.UpdateAsyncWorkflowAckLevel(ctx, &persistence.InternalUpdateAsyncWorkflowAckLevelRequest{
			UpdateAsyncWorkflowAckLevelRequest: &persistence.UpdateAsyncWorkflowAckLevelRequest{ShardID: 3, SourceCluster: "c1", AckLevel: 40},
			CurrentTimeStamp:                   now,
		})
		require.NoError(t, err)
	})

	t.Run("range delete", func(t *testing.T) {
		store, db, sharded, storeShard := newTestAsyncWorkflowQueueStore(t)
		sharded.EXPECT().GetStoreShardByHistoryShard(3).Return(storeShard, nil)
		db.EXPECT().RangeDeleteAsyncWorkflowMessages(ctx, 3, "c1", int64(20)).Return(nil)

		err := store.RangeDeleteAsyncWorkflowMessages(ctx, &persistence.RangeDeleteAsyncWorkflowMessagesRequest{
			ShardID: 3, SourceCluster: "c1", InclusiveMaxMessageID: 20,
		})
		require.NoError(t, err)
	})
}

func TestNoSQLAsyncWorkflowQueueStoreName(t *testing.T) {
	store, db, sharded, storeShard := newTestAsyncWorkflowQueueStore(t)
	sharded.EXPECT().GetDefaultShard().Return(*storeShard)
	db.EXPECT().PluginName().Return("cassandra")
	assert.Equal(t, "cassandra", store.GetName())
}

func TestNoSQLAsyncWorkflowQueueStoreDuplicateMessage(t *testing.T) {
	ctx := context.Background()
	store, db, sharded, storeShard := newTestAsyncWorkflowQueueStore(t)
	sharded.EXPECT().GetStoreShardByHistoryShard(3).Return(storeShard, nil)
	db.EXPECT().InsertAsyncWorkflowMessage(ctx, gomock.Any()).Return(nosqlplugin.NewConditionFailure("asyncWorkflowQueue"))

	err := store.EnqueueAsyncWorkflowMessage(ctx, &persistence.InternalEnqueueAsyncWorkflowMessageRequest{
		EnqueueAsyncWorkflowMessageRequest: &persistence.EnqueueAsyncWorkflowMessageRequest{ShardID: 3, SourceCluster: "c1", MessageID: 7},
	})

	var conditionFailed *persistence.ConditionFailedError
	require.ErrorAs(t, err, &conditionFailed)
	assert.Contains(t, conditionFailed.Msg, "7")
}

func TestNoSQLAsyncWorkflowQueueStoreDLQNotImplemented(t *testing.T) {
	ctx := context.Background()
	store, _, _, _ := newTestAsyncWorkflowQueueStore(t)

	assert.ErrorIs(t, store.EnqueueAsyncWorkflowMessageToDLQ(ctx, &persistence.InternalEnqueueAsyncWorkflowMessageToDLQRequest{}), persistence.ErrAsyncWorkflowQueueNotImplemented)
	_, err := store.ReadAsyncWorkflowMessagesFromDLQ(ctx, &persistence.ReadAsyncWorkflowMessagesFromDLQRequest{})
	assert.ErrorIs(t, err, persistence.ErrAsyncWorkflowQueueNotImplemented)
	assert.ErrorIs(t, store.RangeDeleteAsyncWorkflowMessagesFromDLQ(ctx, &persistence.RangeDeleteAsyncWorkflowMessagesFromDLQRequest{}), persistence.ErrAsyncWorkflowQueueNotImplemented)
}

func TestNoSQLAsyncWorkflowQueueStoreErrors(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2025, 1, 6, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		// expectDB registers the single expected DB call, returning err.
		expectDB func(db *nosqlplugin.MockDB, err error)
		call     func(store *nosqlAsyncWorkflowQueueStore) error
	}{
		{
			name: "EnqueueAsyncWorkflowMessage",
			expectDB: func(db *nosqlplugin.MockDB, err error) {
				db.EXPECT().InsertAsyncWorkflowMessage(ctx, gomock.Any()).Return(err)
			},
			call: func(store *nosqlAsyncWorkflowQueueStore) error {
				return store.EnqueueAsyncWorkflowMessage(ctx, &persistence.InternalEnqueueAsyncWorkflowMessageRequest{
					EnqueueAsyncWorkflowMessageRequest: &persistence.EnqueueAsyncWorkflowMessageRequest{ShardID: 3, SourceCluster: "c1", MessageID: 1},
					CurrentTimeStamp:                   now,
				})
			},
		},
		{
			name: "ReadAsyncWorkflowMessages",
			expectDB: func(db *nosqlplugin.MockDB, err error) {
				db.EXPECT().SelectAsyncWorkflowMessages(ctx, 3, "c1", int64(1), 5).Return(nil, err)
			},
			call: func(store *nosqlAsyncWorkflowQueueStore) error {
				_, err := store.ReadAsyncWorkflowMessages(ctx, &persistence.ReadAsyncWorkflowMessagesRequest{ShardID: 3, SourceCluster: "c1", ExclusiveMinMessageID: 1, PageSize: 5})
				return err
			},
		},
		{
			name: "GetAsyncWorkflowAckLevels",
			expectDB: func(db *nosqlplugin.MockDB, err error) {
				db.EXPECT().SelectAsyncWorkflowAckLevels(ctx, 3).Return(nil, err)
			},
			call: func(store *nosqlAsyncWorkflowQueueStore) error {
				_, err := store.GetAsyncWorkflowAckLevels(ctx, &persistence.GetAsyncWorkflowAckLevelsRequest{ShardID: 3})
				return err
			},
		},
		{
			name: "UpdateAsyncWorkflowAckLevel",
			expectDB: func(db *nosqlplugin.MockDB, err error) {
				db.EXPECT().UpsertAsyncWorkflowAckLevel(ctx, 3, "c1", int64(40), now).Return(err)
			},
			call: func(store *nosqlAsyncWorkflowQueueStore) error {
				return store.UpdateAsyncWorkflowAckLevel(ctx, &persistence.InternalUpdateAsyncWorkflowAckLevelRequest{
					UpdateAsyncWorkflowAckLevelRequest: &persistence.UpdateAsyncWorkflowAckLevelRequest{ShardID: 3, SourceCluster: "c1", AckLevel: 40},
					CurrentTimeStamp:                   now,
				})
			},
		},
		{
			name: "RangeDeleteAsyncWorkflowMessages",
			expectDB: func(db *nosqlplugin.MockDB, err error) {
				db.EXPECT().RangeDeleteAsyncWorkflowMessages(ctx, 3, "c1", int64(20)).Return(err)
			},
			call: func(store *nosqlAsyncWorkflowQueueStore) error {
				return store.RangeDeleteAsyncWorkflowMessages(ctx, &persistence.RangeDeleteAsyncWorkflowMessagesRequest{ShardID: 3, SourceCluster: "c1", InclusiveMaxMessageID: 20})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name+" shard lookup error", func(t *testing.T) {
			store, _, sharded, _ := newTestAsyncWorkflowQueueStore(t)
			lookupErr := errors.New("no such shard")
			sharded.EXPECT().GetStoreShardByHistoryShard(3).Return(nil, lookupErr)

			err := tc.call(store)
			assert.Same(t, lookupErr, err)
		})

		t.Run(tc.name+" db error", func(t *testing.T) {
			store, db, sharded, storeShard := newTestAsyncWorkflowQueueStore(t)
			sharded.EXPECT().GetStoreShardByHistoryShard(3).Return(storeShard, nil)
			tc.expectDB(db, errors.New("boom"))
			db.EXPECT().IsNotFoundError(gomock.Any()).Return(false)
			db.EXPECT().IsTimeoutError(gomock.Any()).Return(false)
			db.EXPECT().IsThrottlingError(gomock.Any()).Return(false)
			db.EXPECT().IsDBUnavailableError(gomock.Any()).Return(false)

			err := tc.call(store)
			var internalErr *types.InternalServiceError
			require.ErrorAs(t, err, &internalErr)
			assert.Contains(t, internalErr.Message, tc.name)
			assert.Contains(t, internalErr.Message, "boom")
		})
	}
}
