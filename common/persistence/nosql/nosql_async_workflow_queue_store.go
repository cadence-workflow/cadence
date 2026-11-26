package nosql

import (
	"context"
	"errors"
	"fmt"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

// nosqlAsyncWorkflowQueueStore backs persistence.AsyncWorkflowQueueStore with the nosqlplugin
// AsyncWorkflowQueueCRUD. The DLQ methods return ErrAsyncWorkflowQueueNotImplemented until the
// DLQ table and its CRUD exist.
type nosqlAsyncWorkflowQueueStore struct {
	shardedNosqlStore
}

var _ persistence.AsyncWorkflowQueueStore = (*nosqlAsyncWorkflowQueueStore)(nil)

func newNoSQLAsyncWorkflowQueueStore(
	cfg config.ShardedNoSQL,
	logger log.Logger,
	metricsClient metrics.Client,
	dc *persistence.DynamicConfiguration,
) (persistence.AsyncWorkflowQueueStore, error) {
	// eagerConnect is false: only the default shard is connected here. Every service builds this
	// store at start, but only history uses it, so the other shards connect on first use.
	shardedStore, err := newShardedNosqlStore(cfg, logger, metricsClient, dc, false)
	if err != nil {
		return nil, err
	}
	return &nosqlAsyncWorkflowQueueStore{shardedNosqlStore: shardedStore}, nil
}

func (sh *nosqlAsyncWorkflowQueueStore) GetName() string {
	return sh.GetDefaultShard().db.PluginName()
}

func (sh *nosqlAsyncWorkflowQueueStore) EnqueueAsyncWorkflowMessage(ctx context.Context, request *persistence.InternalEnqueueAsyncWorkflowMessageRequest) error {
	storeShard, err := sh.GetStoreShardByHistoryShard(request.ShardID)
	if err != nil {
		return err
	}

	err = storeShard.db.InsertAsyncWorkflowMessage(ctx, &nosqlplugin.AsyncWorkflowMessageRow{
		ShardID:         request.ShardID,
		SourceCluster:   request.SourceCluster,
		MessageID:       request.MessageID,
		DomainName:      request.DomainName,
		WorkflowID:      request.WorkflowID,
		RequestID:       request.RequestID,
		RequestType:     int(request.RequestType),
		Payload:         request.Payload,
		PayloadEncoding: string(request.PayloadEncoding),
		CreatedTime:     request.CurrentTimeStamp,
	})
	if err != nil {
		var conditionFailure *nosqlplugin.ConditionFailure
		if errors.As(err, &conditionFailure) {
			return &persistence.ConditionFailedError{
				Msg: fmt.Sprintf("async workflow message %d already exists for shard %d source %q", request.MessageID, request.ShardID, request.SourceCluster),
			}
		}
		return convertCommonErrors(storeShard.db, "EnqueueAsyncWorkflowMessage", err)
	}
	return nil
}

func (sh *nosqlAsyncWorkflowQueueStore) ReadAsyncWorkflowMessages(ctx context.Context, request *persistence.ReadAsyncWorkflowMessagesRequest) (*persistence.ReadAsyncWorkflowMessagesResponse, error) {
	storeShard, err := sh.GetStoreShardByHistoryShard(request.ShardID)
	if err != nil {
		return nil, err
	}

	rows, err := storeShard.db.SelectAsyncWorkflowMessages(ctx, request.ShardID, request.SourceCluster, request.ExclusiveMinMessageID, request.PageSize)
	if err != nil {
		return nil, convertCommonErrors(storeShard.db, "ReadAsyncWorkflowMessages", err)
	}

	messages := make([]*persistence.AsyncWorkflowMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, asyncWorkflowMessageFromRow(row))
	}
	return &persistence.ReadAsyncWorkflowMessagesResponse{Messages: messages}, nil
}

func (sh *nosqlAsyncWorkflowQueueStore) GetAsyncWorkflowAckLevels(ctx context.Context, request *persistence.GetAsyncWorkflowAckLevelsRequest) (*persistence.GetAsyncWorkflowAckLevelsResponse, error) {
	storeShard, err := sh.GetStoreShardByHistoryShard(request.ShardID)
	if err != nil {
		return nil, err
	}

	levels, err := storeShard.db.SelectAsyncWorkflowAckLevels(ctx, request.ShardID)
	if err != nil {
		return nil, convertCommonErrors(storeShard.db, "GetAsyncWorkflowAckLevels", err)
	}
	if levels == nil {
		levels = make(map[string]int64)
	}
	return &persistence.GetAsyncWorkflowAckLevelsResponse{AckLevels: levels}, nil
}

func (sh *nosqlAsyncWorkflowQueueStore) UpdateAsyncWorkflowAckLevel(ctx context.Context, request *persistence.InternalUpdateAsyncWorkflowAckLevelRequest) error {
	storeShard, err := sh.GetStoreShardByHistoryShard(request.ShardID)
	if err != nil {
		return err
	}

	err = storeShard.db.UpsertAsyncWorkflowAckLevel(ctx, request.ShardID, request.SourceCluster, request.AckLevel, request.CurrentTimeStamp)
	if err != nil {
		return convertCommonErrors(storeShard.db, "UpdateAsyncWorkflowAckLevel", err)
	}
	return nil
}

func (sh *nosqlAsyncWorkflowQueueStore) RangeDeleteAsyncWorkflowMessages(ctx context.Context, request *persistence.RangeDeleteAsyncWorkflowMessagesRequest) error {
	storeShard, err := sh.GetStoreShardByHistoryShard(request.ShardID)
	if err != nil {
		return err
	}

	err = storeShard.db.RangeDeleteAsyncWorkflowMessages(ctx, request.ShardID, request.SourceCluster, request.InclusiveMaxMessageID)
	if err != nil {
		return convertCommonErrors(storeShard.db, "RangeDeleteAsyncWorkflowMessages", err)
	}
	return nil
}

func (sh *nosqlAsyncWorkflowQueueStore) EnqueueAsyncWorkflowMessageToDLQ(context.Context, *persistence.InternalEnqueueAsyncWorkflowMessageToDLQRequest) error {
	return persistence.ErrAsyncWorkflowQueueNotImplemented
}

func (sh *nosqlAsyncWorkflowQueueStore) ReadAsyncWorkflowMessagesFromDLQ(context.Context, *persistence.ReadAsyncWorkflowMessagesFromDLQRequest) (*persistence.ReadAsyncWorkflowMessagesFromDLQResponse, error) {
	return nil, persistence.ErrAsyncWorkflowQueueNotImplemented
}

func (sh *nosqlAsyncWorkflowQueueStore) RangeDeleteAsyncWorkflowMessagesFromDLQ(context.Context, *persistence.RangeDeleteAsyncWorkflowMessagesFromDLQRequest) error {
	return persistence.ErrAsyncWorkflowQueueNotImplemented
}

// asyncWorkflowMessageFromRow converts a plugin row into the persistence message. The DLQ store
// reuses it, so keep the signature stable.
func asyncWorkflowMessageFromRow(row *nosqlplugin.AsyncWorkflowMessageRow) *persistence.AsyncWorkflowMessage {
	return &persistence.AsyncWorkflowMessage{
		ShardID:         row.ShardID,
		SourceCluster:   row.SourceCluster,
		MessageID:       row.MessageID,
		DomainName:      row.DomainName,
		WorkflowID:      row.WorkflowID,
		RequestID:       row.RequestID,
		RequestType:     persistence.AsyncWorkflowRequestType(row.RequestType),
		Payload:         row.Payload,
		PayloadEncoding: constants.EncodingType(row.PayloadEncoding),
		CreatedTime:     row.CreatedTime,
	}
}
