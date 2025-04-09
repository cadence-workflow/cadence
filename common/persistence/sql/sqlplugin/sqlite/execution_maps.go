package sqlite

import (
	"context"
	"database/sql"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	createSignalsRequestedSetQry = `INSERT OR IGNORE INTO signals_requested_sets
(shard_id, domain_id, workflow_id, run_id, signal_id) VALUES
(:shard_id, :domain_id, :workflow_id, :run_id, :signal_id)`
)

// InsertIntoSignalsRequestedSets inserts one or more rows into signals_requested_sets table
func (mdb *DB) InsertIntoSignalsRequestedSets(ctx context.Context, rows []sqlplugin.SignalsRequestedSetsRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(rows[0].ShardID), mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, createSignalsRequestedSetQry, rows)
}
