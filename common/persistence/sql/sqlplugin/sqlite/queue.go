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
