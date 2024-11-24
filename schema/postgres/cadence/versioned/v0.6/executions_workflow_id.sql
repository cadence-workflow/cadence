ALTER TABLE executions
ALTER COLUMN workflow_id TYPE text ;

ALTER TABLE current_executions
ALTER COLUMN workflow_id TYPE  text;

ALTER TABLE buffered_events
ALTER COLUMN workflow_id TYPE text;

ALTER TABLE activity_info_maps
ALTER COLUMN workflow_id TYPE text;

ALTER TABLE  timer_info_maps
ALTER COLUMN workflow_id TYPE text;

ALTER TABLE child_execution_info_maps
ALTER COLUMN workflow_id TYPE text;

ALTER TABLE request_cancel_info_maps
ALTER COLUMN workflow_id TYPE  text;

ALTER TABLE signal_info_maps
ALTER COLUMN workflow_id TYPE text;

ALTER TABLE buffered_replication_task_maps
ALTER COLUMN workflow_id TYPE text;

ALTER TABLE signals_requested_sets
ALTER COLUMN workflow_id TYPE text;