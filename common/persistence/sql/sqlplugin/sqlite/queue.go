package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	templateGetLastMessageIDQuery = `SELECT message_id FROM queue WHERE queue_type=? ORDER BY message_id DESC LIMIT 1`
	templateGetQueueMetadataQuery = `SELECT data from queue_metadata WHERE queue_type = ?`
)

// GetLastEnqueuedMessageIDForUpdate returns the last enqueued message ID
func (mdb *DB) GetLastEnqueuedMessageIDForUpdate(
	ctx context.Context,
	queueType persistence.QueueType,
) (int64, error) {

	var lastMessageID int64
	err := mdb.driver.GetContext(ctx, sqlplugin.DbDefaultShard, &lastMessageID, templateGetLastMessageIDQuery, queueType)
	return lastMessageID, err
}

// GetAckLevels returns ack levels for pulling clusters
func (mdb *DB) GetAckLevels(
	ctx context.Context,
	queueType persistence.QueueType,
	forUpdate bool,
) (map[string]int64, error) {

	queryStr := templateGetQueueMetadataQuery
	if forUpdate {
		// FOR UPDATE is not supported in sqlite
	}

	var data []byte
	err := mdb.driver.GetContext(ctx, sqlplugin.DbDefaultShard, &data, queryStr, queueType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, err
	}

	var clusterAckLevels map[string]int64
	if err := json.Unmarshal(data, &clusterAckLevels); err != nil {
		return nil, err
	}

	return clusterAckLevels, nil
}
