// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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
