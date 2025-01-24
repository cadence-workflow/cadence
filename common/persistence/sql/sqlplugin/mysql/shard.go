// Copyright (c) 2017 Uber Technologies, Inc.
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

package mysql

import (
	"context"
	"database/sql"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	createShardQry = `INSERT INTO
 shards (shard_id, range_id, data, data_encoding) VALUES (?, ?, ?, ?)`

	getShardQry = `SELECT
 shard_id, range_id, data, data_encoding
 FROM shards WHERE shard_id = ?`

	updateShardQry = `UPDATE shards 
 SET range_id = ?, data = ?, data_encoding = ? 
 WHERE shard_id = ?`

	lockShardQry     = `SELECT range_id FROM shards WHERE shard_id = ? FOR UPDATE`
	readLockShardQry = `SELECT range_id FROM shards WHERE shard_id = ? LOCK IN SHARE MODE`
)

// InsertIntoShards inserts one or more rows into shards table
func (mdb *DB) InsertIntoShards(ctx context.Context, row *sqlplugin.ShardsRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(row.ShardID), mdb.GetTotalNumDBShards())
	return mdb.driver.ExecContext(ctx, dbShardID, createShardQry, row.ShardID, row.RangeID, row.Data, row.DataEncoding)
}

// UpdateShards updates one or more rows into shards table
func (mdb *DB) UpdateShards(ctx context.Context, row *sqlplugin.ShardsRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(row.ShardID), mdb.GetTotalNumDBShards())
	return mdb.driver.ExecContext(ctx, dbShardID, updateShardQry, row.RangeID, row.Data, row.DataEncoding, row.ShardID)
}

// SelectFromShards reads one or more rows from shards table
func (mdb *DB) SelectFromShards(ctx context.Context, filter *sqlplugin.ShardsFilter) (*sqlplugin.ShardsRow, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	var row sqlplugin.ShardsRow
	err := mdb.driver.GetContext(ctx, dbShardID, &row, getShardQry, filter.ShardID)
	if err != nil {
		return nil, err
	}
	return &row, err
}

// ReadLockShards acquires a read lock on a single row in shards table
func (mdb *DB) ReadLockShards(ctx context.Context, filter *sqlplugin.ShardsFilter) (int, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	var rangeID int
	err := mdb.driver.GetContext(ctx, dbShardID, &rangeID, readLockShardQry, filter.ShardID)
	return rangeID, err
}

// WriteLockShards acquires a write lock on a single row in shards table
func (mdb *DB) WriteLockShards(ctx context.Context, filter *sqlplugin.ShardsFilter) (int, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	var rangeID int
	err := mdb.driver.GetContext(ctx, dbShardID, &rangeID, lockShardQry, filter.ShardID)
	return rangeID, err
}
