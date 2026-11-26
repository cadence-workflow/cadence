package cassandra

import (
	"context"
	"time"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

const (
	asyncWorkflowRowTypeMessage = 0
	asyncWorkflowRowTypeControl = 1

	// asyncWorkflowControlMessageID is negative so it cannot collide with real,
	// monotonically increasing message ids.
	asyncWorkflowControlMessageID = -12345
)

// InsertAsyncWorkflowMessage inserts a message row with IF NOT EXISTS. A row that already
// exists under the same (shard, source cluster, message id) is left untouched and reported as
// a ConditionFailure.
func (db *CDB) InsertAsyncWorkflowMessage(ctx context.Context, row *nosqlplugin.AsyncWorkflowMessageRow) error {
	query := db.session.Query(templateInsertAsyncWorkflowMessageQuery,
		row.ShardID,
		asyncWorkflowRowTypeMessage,
		row.SourceCluster,
		row.MessageID,
		row.Payload,
		row.PayloadEncoding,
		row.DomainName,
		row.WorkflowID,
		row.RequestID,
		row.RequestType,
		row.CreatedTime,
	).WithContext(ctx)
	applied, err := query.MapScanCAS(make(map[string]interface{}))
	if err != nil {
		return err
	}
	if !applied {
		return nosqlplugin.NewConditionFailure("asyncWorkflowQueue")
	}
	return nil
}

// SelectAsyncWorkflowMessages returns message rows after the exclusive lower bound.
func (db *CDB) SelectAsyncWorkflowMessages(
	ctx context.Context,
	shardID int,
	sourceCluster string,
	exclusiveMinMessageID int64,
	pageSize int,
) ([]*nosqlplugin.AsyncWorkflowMessageRow, error) {
	query := db.session.Query(templateSelectAsyncWorkflowMessagesQuery,
		shardID,
		asyncWorkflowRowTypeMessage,
		sourceCluster,
		exclusiveMinMessageID,
	).PageSize(pageSize).WithContext(ctx)

	iter := query.Iter()
	if iter == nil {
		return nil, &types.InternalServiceError{Message: "SelectAsyncWorkflowMessages operation failed. Not able to create query iterator."}
	}

	var rows []*nosqlplugin.AsyncWorkflowMessageRow
	for len(rows) < pageSize {
		row := &nosqlplugin.AsyncWorkflowMessageRow{
			ShardID:       shardID,
			SourceCluster: sourceCluster,
		}
		if !iter.Scan(
			&row.MessageID,
			&row.Payload,
			&row.PayloadEncoding,
			&row.DomainName,
			&row.WorkflowID,
			&row.RequestID,
			&row.RequestType,
			&row.CreatedTime,
		) {
			break
		}
		rows = append(rows, row)
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}
	return rows, nil
}

// SelectAsyncWorkflowAckLevels returns every source cluster's control-row ack level for a shard.
func (db *CDB) SelectAsyncWorkflowAckLevels(ctx context.Context, shardID int) (map[string]int64, error) {
	query := db.session.Query(templateSelectAsyncWorkflowAckLevelsQuery,
		shardID,
		asyncWorkflowRowTypeControl,
	).WithContext(ctx)

	iter := query.Iter()
	if iter == nil {
		return nil, &types.InternalServiceError{Message: "SelectAsyncWorkflowAckLevels operation failed. Not able to create query iterator."}
	}

	levels := make(map[string]int64)
	var sourceCluster string
	var ackLevel int64
	for iter.Scan(&sourceCluster, &ackLevel) {
		levels[sourceCluster] = ackLevel
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return levels, nil
}

// UpsertAsyncWorkflowAckLevel creates a control row or monotonically advances its ack level.
func (db *CDB) UpsertAsyncWorkflowAckLevel(
	ctx context.Context,
	shardID int,
	sourceCluster string,
	ackLevel int64,
	now time.Time,
) error {
	insert := db.session.Query(templateInsertAsyncWorkflowAckLevelQuery,
		shardID,
		asyncWorkflowRowTypeControl,
		sourceCluster,
		asyncWorkflowControlMessageID,
		ackLevel,
		now,
		now,
	).WithContext(ctx)
	applied, err := insert.MapScanCAS(make(map[string]interface{}))
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	update := db.session.Query(templateUpdateAsyncWorkflowAckLevelQuery,
		ackLevel,
		now,
		shardID,
		asyncWorkflowRowTypeControl,
		sourceCluster,
		asyncWorkflowControlMessageID,
		ackLevel,
	).WithContext(ctx)
	_, err = update.MapScanCAS(make(map[string]interface{}))
	return err
}

// RangeDeleteAsyncWorkflowMessages deletes message rows through the inclusive bound.
// It is scoped to type=0 so the control row is never deleted. Like the base tasks table,
// the range delete is deliberately unfenced.
func (db *CDB) RangeDeleteAsyncWorkflowMessages(ctx context.Context, shardID int, sourceCluster string, inclusiveMaxMessageID int64) error {
	query := db.session.Query(templateRangeDeleteAsyncWorkflowMessagesQuery,
		shardID,
		asyncWorkflowRowTypeMessage,
		sourceCluster,
		inclusiveMaxMessageID,
	).WithContext(ctx)
	return db.executeWithConsistencyAll(query)
}
