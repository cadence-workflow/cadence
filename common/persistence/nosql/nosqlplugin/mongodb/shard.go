// Copyright (c) 2020 Uber Technologies, Inc.
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

package mongodb

import (
	"context"
	"log"
	"time"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

// InsertShard creates a new shard, return error is there is any.
// Return ShardOperationConditionFailure if the condition doesn't meet
func (db *mdb) InsertShard(ctx context.Context, row *nosqlplugin.ShardRow, timeStamp time.Time) error {
	log.Println("not implemented...ignore the error for testing...")
	return nil
}

// SelectShard gets a shard
func (db *mdb) SelectShard(ctx context.Context, shardID int, currentClusterName string) (int64, *nosqlplugin.ShardRow, error) {
	panic("TODO")
}

// UpdateRangeID updates the rangeID, return error is there is any
// Return ShardOperationConditionFailure if the condition doesn't meet
func (db *mdb) UpdateRangeID(ctx context.Context, shardID int, rangeID int64, previousRangeID int64) error {
	panic("TODO")
}

// UpdateShard updates a shard, return error is there is any.
// Return ShardOperationConditionFailure if the condition doesn't meet
func (db *mdb) UpdateShard(ctx context.Context, row *nosqlplugin.ShardRow, previousRangeID int64, timeStamp time.Time) error {
	panic("TODO")
}
