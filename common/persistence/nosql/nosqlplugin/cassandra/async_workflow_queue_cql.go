// Copyright (c) 2026 Uber Technologies, Inc.
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
	// async_workflow_queue: enqueue is a plain insert (the shard owner is the single writer, so message_id
	// is a monotonic per-shard sequence and no conditional write is required).
	templateEnqueueAsyncWorkflowMessageQuery      = `INSERT INTO async_workflow_queue (shard_id, message_id, message_payload, encoding, partition_key, created_time) VALUES(?, ?, ?, ?, ?, ?)`
	templateGetLastAsyncWorkflowMessageIDQuery    = `SELECT message_id FROM async_workflow_queue WHERE shard_id=? ORDER BY message_id DESC LIMIT 1`
	templateGetAsyncWorkflowMessagesFromQuery     = `SELECT message_id, message_payload, encoding, partition_key, created_time FROM async_workflow_queue WHERE shard_id=? AND message_id > ? LIMIT ?`
	templateRangeDeleteAsyncWorkflowMessagesQuery = `DELETE FROM async_workflow_queue WHERE shard_id=? AND message_id <= ?`

	// async_workflow_queue_metadata: per-shard ack level with CAS versioning.
	templateGetAsyncWorkflowQueueMetadataQuery    = `SELECT ack_level, version FROM async_workflow_queue_metadata WHERE shard_id=?`
	templateInsertAsyncWorkflowQueueMetadataQuery = `INSERT INTO async_workflow_queue_metadata (shard_id, ack_level, version, created_time) VALUES(?, ?, ?, ?) IF NOT EXISTS`
	templateUpdateAsyncWorkflowQueueMetadataQuery = `UPDATE async_workflow_queue_metadata SET ack_level = ?, version = ?, last_updated_time = ? WHERE shard_id=? IF version = ?`

	// async_workflow_queue_dlq: poison messages, same shape as async_workflow_queue.
	templateEnqueueAsyncWorkflowDLQMessageQuery      = `INSERT INTO async_workflow_queue_dlq (shard_id, message_id, message_payload, encoding, partition_key, created_time) VALUES(?, ?, ?, ?, ?, ?)`
	templateGetLastAsyncWorkflowDLQMessageIDQuery    = `SELECT message_id FROM async_workflow_queue_dlq WHERE shard_id=? ORDER BY message_id DESC LIMIT 1`
	templateGetAsyncWorkflowDLQMessagesFromQuery     = `SELECT message_id, message_payload, encoding, partition_key, created_time FROM async_workflow_queue_dlq WHERE shard_id=? AND message_id > ? LIMIT ?`
	templateRangeDeleteAsyncWorkflowDLQMessagesQuery = `DELETE FROM async_workflow_queue_dlq WHERE shard_id=? AND message_id <= ?`
)
