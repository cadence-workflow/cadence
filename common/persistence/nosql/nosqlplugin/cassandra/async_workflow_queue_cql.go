package cassandra

const (
	templateInsertAsyncWorkflowMessageQuery = `INSERT INTO async_workflow_queue ` +
		`(shard_id, type, source_cluster, message_id, message_payload, message_payload_encoding, domain_name, workflow_id, request_id, request_type, created_time) ` +
		`VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`
	templateSelectAsyncWorkflowMessagesQuery = `SELECT message_id, message_payload, message_payload_encoding, domain_name, workflow_id, request_id, request_type, created_time ` +
		`FROM async_workflow_queue ` +
		`WHERE shard_id = ? AND type = ? AND source_cluster = ? AND message_id > ?`
	templateSelectAsyncWorkflowAckLevelsQuery = `SELECT source_cluster, ack_level ` +
		`FROM async_workflow_queue ` +
		`WHERE shard_id = ? AND type = ?`
	templateInsertAsyncWorkflowAckLevelQuery = `INSERT INTO async_workflow_queue ` +
		`(shard_id, type, source_cluster, message_id, ack_level, created_time, last_updated_time) ` +
		`VALUES (?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`
	templateUpdateAsyncWorkflowAckLevelQuery = `UPDATE async_workflow_queue SET ack_level = ?, last_updated_time = ? ` +
		`WHERE shard_id = ? AND type = ? AND source_cluster = ? AND message_id = ? ` +
		`IF ack_level < ?`
	templateRangeDeleteAsyncWorkflowMessagesQuery = `DELETE FROM async_workflow_queue ` +
		`WHERE shard_id = ? AND type = ? AND source_cluster = ? AND message_id <= ?`
)
