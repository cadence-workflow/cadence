package sqlite

import (
	"context"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	lockShardQry     = `SELECT range_id FROM shards WHERE shard_id = ?`
	readLockShardQry = `SELECT range_id FROM shards WHERE shard_id = ?`
)

// WriteLockShards acquires a write lock on a single row in shards table
func (mdb *DB) WriteLockShards(ctx context.Context, filter *sqlplugin.ShardsFilter) (int, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	var rangeID int
	err := mdb.driver.GetContext(ctx, dbShardID, &rangeID, lockShardQry, filter.ShardID)
	return rangeID, err
}

// ReadLockShards acquires a read lock on a single row in shards table
func (mdb *DB) ReadLockShards(ctx context.Context, filter *sqlplugin.ShardsFilter) (int, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	var rangeID int
	err := mdb.driver.GetContext(ctx, dbShardID, &rangeID, readLockShardQry, filter.ShardID)
	return rangeID, err
}
