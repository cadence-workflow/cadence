// Copyright (c) 2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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

package cassandra

const (
	templateEnqueueMessageQuery             = `INSERT INTO queue (queue_type, message_id, message_payload, created_time) VALUES(?, ?, ?, ?) IF NOT EXISTS`
	templateGetLastMessageIDQuery           = `SELECT message_id FROM queue WHERE queue_type=? ORDER BY message_id DESC LIMIT 1`
	templateGetMessagesQuery                = `SELECT message_id, message_payload FROM queue WHERE queue_type = ? and message_id > ? LIMIT ?`
	templateGetMessagesFromDLQQuery         = `SELECT message_id, message_payload FROM queue WHERE queue_type = ? and message_id > ? and message_id <= ?`
	templateRangeDeleteMessagesBeforeQuery  = `DELETE FROM queue WHERE queue_type = ? and message_id < ?`
	templateRangeDeleteMessagesBetweenQuery = `DELETE FROM queue WHERE queue_type = ? and message_id > ? and message_id <= ?`
	templateDeleteMessageQuery              = `DELETE FROM queue WHERE queue_type = ? and message_id = ?`
	templateGetQueueMetadataQuery           = `SELECT cluster_ack_level, version FROM queue_metadata WHERE queue_type = ?`
	templateInsertQueueMetadataQuery        = `INSERT INTO queue_metadata (queue_type, cluster_ack_level, version, created_time) VALUES(?, ?, ?, ?) IF NOT EXISTS`
	templateUpdateQueueMetadataQuery        = `UPDATE queue_metadata SET cluster_ack_level = ?, version = ?, last_updated_time = ? WHERE queue_type = ? IF version = ?`
	templateGetQueueSizeQuery               = `SELECT COUNT(1) AS count FROM queue WHERE queue_type=?`
)
